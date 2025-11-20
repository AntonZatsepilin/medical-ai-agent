import React, { useState, useEffect, useRef } from 'react';

const VoiceChat: React.FC = () => {
  const [isListening, setIsListening] = useState(false);
  const [isHandsFree, setIsHandsFree] = useState(true); // Default to true as requested
  const [messages, setMessages] = useState<{role: string, text: string}[]>([]);
  
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const consultationIdRef = useRef<string | null>(null);
  const isProcessingRef = useRef(false);
  const audioContextRef = useRef<AudioContext | null>(null);
  const silenceTimerRef = useRef<any>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const animationFrameRef = useRef<number | null>(null);

  const isManualStop = useRef(false);
  const isHandsFreeRef = useRef(isHandsFree);
  const audioQueueRef = useRef<string[]>([]);
  const isPlayingRef = useRef(false);
  const isStreamDoneRef = useRef(false);
  const streamRef = useRef<MediaStream | null>(null);

  useEffect(() => {
    isHandsFreeRef.current = isHandsFree;
  }, [isHandsFree]);

  const [isSpeaking, setIsSpeaking] = useState(false); // Visual feedback for VAD


  useEffect(() => {
    // Initialize Consultation on mount
    createConsultation();

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

  const startListening = async () => {
      try {
        initAudioContext();
        const audioContext = audioContextRef.current!;
        
        let stream = streamRef.current;
        if (!stream || !stream.active) {
             stream = await navigator.mediaDevices.getUserMedia({ audio: true });
             streamRef.current = stream;
        }
        
        // VAD Setup
        // Reuse analyser if possible, or create new one
        let analyser = analyserRef.current;
        if (!analyser) {
            const source = audioContext.createMediaStreamSource(stream);
            analyser = audioContext.createAnalyser();
            analyser.fftSize = 512;
            analyser.minDecibels = -80; 
            analyser.smoothingTimeConstant = 0.85; 
            source.connect(analyser);
            analyserRef.current = analyser;
        }
        
        const bufferLength = analyser.frequencyBinCount;
        const dataArray = new Uint8Array(bufferLength);
        
        let lastSpeechTime = Date.now();
        let startTime = Date.now();
        let hasSpoken = false;
        
        const checkSilence = () => {
            if (mediaRecorderRef.current?.state !== 'recording') return;
            
            analyser!.getByteFrequencyData(dataArray);
            
            // Calculate average volume for speech frequencies (approx 300Hz - 3400Hz)
            // Bin width ~93Hz (48000/512)
            const startBin = 3; // ~280Hz
            const endBin = 40;  // ~3700Hz
            
            let sum = 0;
            for(let i = startBin; i < endBin; i++) {
                sum += dataArray[i];
            }
            const average = sum / (endBin - startBin);
            
            // Threshold for speech detection
            // Frequency data is 0-255. 
            // Background noise is usually < 10-15 in these bands.
            // Speech is usually > 30-40.
            // Lowered to 2 for extreme sensitivity as requested
            const SPEECH_THRESHOLD = 1; 
            
            if (average > SPEECH_THRESHOLD) { 
                lastSpeechTime = Date.now();
                if (!hasSpoken) {
                    console.log("Speech detected! Level:", average.toFixed(1));
                    hasSpoken = true;
                    setIsSpeaking(true);
                }
            } else {
                if (Date.now() - lastSpeechTime > 500) {
                    setIsSpeaking(false);
                }
            }
            
            // If silence for 1.5s AND we have detected speech previously
            if (hasSpoken && (Date.now() - lastSpeechTime > 1500)) {
                console.log("Silence detected, stopping...");
                stopListening();
                return; 
            }
            
            // Safety timeout: stop after 15 seconds max
            if (Date.now() - startTime > 15000) {
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
            setIsSpeaking(false);
            
            // Do NOT disconnect source/analyser here if we want to reuse them
            // source.disconnect();
            // analyser.disconnect();
            
            const blob = new Blob(chunksRef.current, { type: 'audio/wav' });
            if (blob.size > 0) {
                // Only upload if we actually detected speech or the file is big enough
                if (hasSpoken || blob.size > 10000) {
                    await handleAudioUpload(blob);
                } else {
                    console.log("Audio too short or empty, ignoring.");
                    setIsListening(false);
                    // Check isManualStop.current BEFORE restarting
                    if (isHandsFree && !isManualStop.current) {
                        setTimeout(() => startListening(), 50); // Reduced delay
                    }
                }
            }
            
            // Only stop tracks if we are manually stopping or leaving the page
            if (isManualStop.current) {
                 stream.getTracks().forEach((track: MediaStreamTrack) => track.stop());
                 streamRef.current = null;
                 analyserRef.current = null;
            }
            
            // IMPORTANT: Do NOT reset isManualStop.current here. 
            // It should only be reset when the user explicitly clicks "Start".
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
      if (mediaRecorderRef.current && mediaRecorderRef.current.state === 'recording') {
          mediaRecorderRef.current.stop();
          setIsListening(false);
      }
  };

  const playNextAudioChunk = async () => {
      if (audioQueueRef.current.length === 0) {
          isPlayingRef.current = false;
          if (isStreamDoneRef.current) {
               isProcessingRef.current = false;
               if (isHandsFreeRef.current && !isManualStop.current) {
                   setTimeout(() => startListening(), 200);
               }
          }
          return;
      }
      
      isPlayingRef.current = true;
      const base64 = audioQueueRef.current.shift();
      if (base64) {
          await playBase64Audio(base64, () => {
              playNextAudioChunk();
          });
      }
  };

  const handleStreamEvent = (event: any) => {
      if (event.type === 'user_text') {
           setMessages((prev: {role: string, text: string}[]) => [...prev, { role: 'user', text: event.data }]);
      } else if (event.type === 'text') {
           setMessages((prev: {role: string, text: string}[]) => {
               const last = prev[prev.length - 1];
               if (last && last.role === 'assistant') {
                   return [...prev.slice(0, -1), { ...last, text: last.text + event.data }];
               } else {
                   return [...prev, { role: 'assistant', text: event.data }];
               }
           });
      } else if (event.type === 'audio') {
           audioQueueRef.current.push(event.data);
           if (!isPlayingRef.current) {
               playNextAudioChunk();
           }
      } else if (event.type === 'done') {
           isStreamDoneRef.current = true;
           if (!isPlayingRef.current) {
               // If nothing is playing, we are truly done
               isProcessingRef.current = false;
               if (isHandsFreeRef.current && !isManualStop.current) {
                   setTimeout(() => startListening(), 200);
               }
           }
      } else if (event.type === 'error') {
           console.error("Stream error:", event.data);
           isProcessingRef.current = false;
      }
  };

  const handleAudioUpload = async (audioBlob: Blob) => {
    if (isProcessingRef.current) return;
    isProcessingRef.current = true;
    isStreamDoneRef.current = false;
    audioQueueRef.current = [];

    if (!consultationIdRef.current) {
        isProcessingRef.current = false;
        return;
    }
    
    const formData = new FormData();
    formData.append('audio', audioBlob);
    formData.append('consultation_id', consultationIdRef.current);

    try {
        const response = await fetch('/api/consultation/audio/stream', {
            method: 'POST',
            body: formData,
        });

        const reader = response.body?.getReader();
        if (!reader) {
             throw new Error("No reader");
        }
        
        const decoder = new TextDecoder();
        let buffer = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            const chunk = decoder.decode(value, { stream: true });
            buffer += chunk;
            
            const lines = buffer.split('\n\n');
            buffer = lines.pop() || ''; 
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const dataStr = line.slice(6);
                    try {
                        const event = JSON.parse(dataStr);
                        handleStreamEvent(event);
                    } catch (e) {
                        console.error("Error parsing SSE event", e);
                    }
                }
            }
        }
    } catch (error) {
        console.error("Error uploading audio", error);
        isProcessingRef.current = false;
        if (isHandsFreeRef.current && !isManualStop.current) {
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
      isManualStop.current = true;
      stopListening();
    } else {
      isManualStop.current = false;
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
                <span className={`h-3 w-3 rounded-full animate-ping ${isSpeaking ? 'bg-green-400' : 'bg-white'}`}></span>
                {isSpeaking ? 'Голос обнаружен...' : 'Слушаю вас...'}
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
