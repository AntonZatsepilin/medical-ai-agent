import React, { useState, useEffect, useRef } from 'react';

const VoiceChat: React.FC = () => {
  const [isListening, setIsListening] = useState(false);
  const [isHandsFree, setIsHandsFree] = useState(true); // Default to true as requested
  const [messages, setMessages] = useState<{role: string, text: string}[]>([]);
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([]);
  
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const consultationIdRef = useRef<string | null>(null);
  const isProcessingRef = useRef(false);
  const audioContextRef = useRef<AudioContext | null>(null);
  const silenceTimerRef = useRef<any>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const animationFrameRef = useRef<number | null>(null);

  useEffect(() => {
    // Initialize Consultation on mount
    createConsultation();

    // Load Voices
    const loadVoices = () => {
      const availableVoices = window.speechSynthesis.getVoices();
      setVoices(availableVoices.filter(v => v.lang.includes('ru')));
    };
    
    window.speechSynthesis.onvoiceschanged = loadVoices;
    loadVoices();

    return () => {
        if (animationFrameRef.current) cancelAnimationFrame(animationFrameRef.current);
        if (silenceTimerRef.current) clearTimeout(silenceTimerRef.current);
    };
  }, []);

  const initAudioContext = () => {
    if (!audioContextRef.current) {
      const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext;
      audioContextRef.current = new AudioContextClass();
    }
    if (audioContextRef.current.state === 'suspended') {
      audioContextRef.current.resume();
    }
  };

  const createConsultation = async () => {
    try {
      const res = await fetch('/api/consultation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ patient_id: "550e8400-e29b-41d4-a716-446655440000" }), // Demo Patient ID
      });
      const data = await res.json();
      consultationIdRef.current = data.consultation_id;
    } catch (error) {
      console.error("Failed to create consultation", error);
    }
  };

  const speakResponse = async (text: string, onEnd?: () => void) => {
    // Try ElevenLabs TTS via Backend
    try {
        console.log("Requesting TTS from backend...");
        const res = await fetch('/api/tts', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text }),
        });

        if (!res.ok) {
            throw new Error(`TTS API failed with status ${res.status}`);
        }

        const blob = await res.blob();
        console.log("TTS Blob received, size:", blob.size);
        
        if (blob.size < 1000) {
             throw new Error("TTS Blob too small, likely error");
        }

        // Use Web Audio API for playback to avoid Autoplay Policy issues
        if (!audioContextRef.current) {
             initAudioContext();
        }
        
        const ctx = audioContextRef.current!;
        const arrayBuffer = await blob.arrayBuffer();
        const audioBuffer = await ctx.decodeAudioData(arrayBuffer);
        
        const source = ctx.createBufferSource();
        source.buffer = audioBuffer;
        source.connect(ctx.destination);
        
        source.onended = () => {
            if (onEnd) onEnd();
        };
        
        source.start(0);
        console.log("Audio playback started via Web Audio API");
        return;

    } catch (e) {
        console.warn("Falling back to browser TTS due to error:", e);
        // Fallback to Browser TTS
        window.speechSynthesis.cancel();
        const utterance = new SpeechSynthesisUtterance(text);
        utterance.lang = 'ru-RU';
        
        const preferredVoice = voices.find((v: SpeechSynthesisVoice) => v.name.includes('Google') && v.lang.includes('ru')) 
                            || voices.find((v: SpeechSynthesisVoice) => v.lang.includes('ru'));
        
        if (preferredVoice) {
            utterance.voice = preferredVoice;
        }

        utterance.pitch = 1.0;
        utterance.rate = 1.1;

        utterance.onend = () => {
            if (onEnd) onEnd();
        };

        window.speechSynthesis.speak(utterance);
    }
  };

  const startListening = async () => {
      try {
        initAudioContext();
        const audioContext = audioContextRef.current!;
        
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        
        // VAD Setup
        const source = audioContext.createMediaStreamSource(stream);
        const analyser = audioContext.createAnalyser();
        analyser.fftSize = 512;
        source.connect(analyser);
        analyserRef.current = analyser;
        
        const bufferLength = analyser.frequencyBinCount;
        const dataArray = new Uint8Array(bufferLength);
        
        let lastSpeechTime = Date.now();
        let startTime = Date.now();
        let hasSpoken = false;
        
        const checkSilence = () => {
            if (mediaRecorderRef.current?.state !== 'recording') return;
            
            // Use TimeDomainData for better volume detection (RMS)
            analyser.getByteTimeDomainData(dataArray);
            
            let sum = 0;
            for(let i = 0; i < bufferLength; i++) {
                const x = dataArray[i] - 128;
                sum += x * x;
            }
            const rms = Math.sqrt(sum / bufferLength);
            
            // Thresholds
            const SPEECH_THRESHOLD = 8; // Sensitivity
            const SILENCE_DURATION = 1000; // Wait 1s of silence
            
            if (rms > SPEECH_THRESHOLD) { 
                lastSpeechTime = Date.now();
                if (!hasSpoken) {
                    console.log("Speech detected! Volume:", rms.toFixed(1));
                    hasSpoken = true;
                }
            }
            
            // If silence detected AFTER speech started
            if (hasSpoken && (Date.now() - lastSpeechTime > SILENCE_DURATION)) {
                console.log("Silence detected, stopping...");
                stopListening();
                return; 
            }
            
            // Safety timeout: stop after 10 seconds max
            if (Date.now() - startTime > 10000) {
                console.log("Max duration reached, stopping...");
                stopListening();
                return;
            }
            
            animationFrameRef.current = requestAnimationFrame(checkSilence);
        };

        const mediaRecorder = new MediaRecorder(stream);
        mediaRecorderRef.current = mediaRecorder;
        chunksRef.current = [];

        mediaRecorder.ondataavailable = (e) => {
            if (e.data.size > 0) {
                chunksRef.current.push(e.data);
            }
        };

        mediaRecorder.onstop = async () => {
            if (animationFrameRef.current) cancelAnimationFrame(animationFrameRef.current);
            
            // Clean up VAD
            source.disconnect();
            analyser.disconnect();
            
            const blob = new Blob(chunksRef.current, { type: 'audio/wav' });
            if (blob.size > 0) {
                // Only upload if we actually detected speech or the file is big enough
                if (hasSpoken || blob.size > 5000) {
                    await handleAudioUpload(blob);
                } else {
                    console.log("Audio too short or empty, ignoring.");
                    setIsListening(false);
                    if (isHandsFree) startListening(); // Restart if it was just noise
                }
            }
            // Stop all tracks
            stream.getTracks().forEach(track => track.stop());
        };

        mediaRecorder.start();
        setIsListening(true);
        checkSilence();
        
      } catch (e) {
          console.error("Error accessing microphone:", e);
          alert("Не удалось получить доступ к микрофону.");
          setIsListening(false);
      }
  };

  const stopListening = () => {
      if (mediaRecorderRef.current && isListening) {
          mediaRecorderRef.current.stop();
          setIsListening(false);
      }
  };

  const handleAudioUpload = async (audioBlob: Blob) => {
    if (isProcessingRef.current) return;
    isProcessingRef.current = true;

    if (!consultationIdRef.current) {
        isProcessingRef.current = false;
        return;
    }

    // Optimistic UI update (optional, maybe show "Processing...")
    
    const formData = new FormData();
    formData.append('audio', audioBlob);
    formData.append('consultation_id', consultationIdRef.current);

    try {
        const res = await fetch('/api/consultation/audio', {
            method: 'POST',
            body: formData,
        });

        const data = await res.json();
        
        if (data.text) {
             setMessages((prev: {role: string, text: string}[]) => [...prev, { role: 'user', text: data.text }]);
        }
        
        if (data.response) {
            const aiResponse = data.response;
            setMessages((prev: {role: string, text: string}[]) => [...prev, { role: 'assistant', text: aiResponse }]);
            
            const onPlaybackEnd = () => {
                isProcessingRef.current = false;
                if (isHandsFree) {
                    setTimeout(() => startListening(), 200);
                }
            };

            if (data.audio_base64) {
                playBase64Audio(data.audio_base64, onPlaybackEnd);
            } else {
                speakResponse(aiResponse, onPlaybackEnd);
            }
        } else {
             isProcessingRef.current = false;
             if (isHandsFree) {
                 startListening();
             }
        }

    } catch (error) {
        console.error("Error uploading audio", error);
        isProcessingRef.current = false;
        // Restart loop if hands-free
        if (isHandsFree) {
             setTimeout(() => startListening(), 1000);
        }
    }
  };

  const playBase64Audio = async (base64String: string, onEnd?: () => void) => {
      try {
        if (!audioContextRef.current) initAudioContext();
        const ctx = audioContextRef.current!;
        
        // Convert base64 to array buffer
        const binaryString = window.atob(base64String);
        const len = binaryString.length;
        const bytes = new Uint8Array(len);
        for (let i = 0; i < len; i++) {
            bytes[i] = binaryString.charCodeAt(i);
        }
        
        const audioBuffer = await ctx.decodeAudioData(bytes.buffer);
        const source = ctx.createBufferSource();
        source.buffer = audioBuffer;
        source.connect(ctx.destination);
        
        source.onended = () => {
            if (onEnd) onEnd();
        };
        
        source.start(0);
      } catch (e) {
          console.error("Error playing base64 audio", e);
          if (onEnd) onEnd();
      }
  };

  const toggleRecording = () => {
    initAudioContext();
    if (isListening) {
      stopListening();
    } else {
      startListening();
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-50 flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-2xl bg-white rounded-2xl shadow-xl overflow-hidden flex flex-col h-[80vh]">
        
        {/* Header */}
        <div className="bg-indigo-600 p-6 text-white flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold">Медицинский Ассистент</h2>
            <p className="text-indigo-100 text-sm">Ваш персональный помощник здоровья</p>
          </div>
          <div className="flex items-center gap-4">
             <div className="flex items-center gap-2 bg-indigo-700 px-3 py-1 rounded-full text-xs cursor-pointer" onClick={() => setIsHandsFree(!isHandsFree)}>
                <div className={`w-2 h-2 rounded-full ${isHandsFree ? 'bg-green-400' : 'bg-gray-400'}`}></div>
                {isHandsFree ? 'Hands-Free' : 'Push-to-Talk'}
             </div>
             <div className="h-10 w-10 bg-white/20 rounded-full flex items-center justify-center">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
                </svg>
             </div>
          </div>
        </div>

        {/* Chat Area */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4 bg-gray-50">
          {messages.length === 0 && (
            <div className="text-center text-gray-400 mt-20">
              <p className="text-lg">
                  {isHandsFree ? "Скажите 'Привет', чтобы начать" : "Нажмите кнопку микрофона, чтобы начать общение"}
              </p>
            </div>
          )}
          
          {messages.map((m, i) => (
            <div key={i} className={`flex ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              <div className={`max-w-[80%] p-4 rounded-2xl shadow-sm ${
                m.role === 'user' 
                  ? 'bg-indigo-600 text-white rounded-br-none' 
                  : 'bg-white text-gray-800 rounded-bl-none border border-gray-100'
              }`}>
                <p className="text-xs opacity-70 mb-1 font-medium uppercase tracking-wider">
                  {m.role === 'user' ? 'Вы' : 'Ассистент'}
                </p>
                <p className="leading-relaxed">{m.text}</p>
              </div>
            </div>
          ))}
        </div>
        
        {/* Controls */}
        <div className="p-6 bg-white border-t border-gray-100">
          <button 
            onClick={toggleRecording}
            className={`w-full py-4 rounded-xl font-bold text-lg shadow-lg transition-all transform hover:scale-[1.02] active:scale-[0.98] flex items-center justify-center gap-3 ${
              isListening 
                ? 'bg-red-500 hover:bg-red-600 text-white animate-pulse' 
                : 'bg-indigo-600 hover:bg-indigo-700 text-white'
            }`}
          >
            {isListening ? (
              <>
                <span className="h-3 w-3 bg-white rounded-full animate-ping"></span>
                Слушаю вас...
              </>
            ) : (
              <>
                <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
                </svg>
                {isHandsFree ? 'Начать диалог' : 'Нажмите, чтобы говорить'}
              </>
            )}
          </button>
          <p className="text-center text-gray-400 text-xs mt-3">
            {isHandsFree 
                ? "Режим Hands-Free включен. Ассистент будет слушать вас автоматически после своего ответа." 
                : "Нажмите кнопку, чтобы записать ответ."}
          </p>
        </div>
      </div>
    </div>
  );
};

export default VoiceChat;
