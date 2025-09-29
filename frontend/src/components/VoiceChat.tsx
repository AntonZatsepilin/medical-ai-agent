import React, { useState, useEffect, useRef } from 'react';

// Types for Web Speech API (since they might not be in standard TS lib)
interface IWindow extends Window {
  webkitSpeechRecognition: any;
  SpeechRecognition: any;
}

const VoiceChat: React.FC = () => {
  const [isListening, setIsListening] = useState(false);
  const [messages, setMessages] = useState<{role: string, text: string}[]>([]);
  const recognitionRef = useRef<any>(null);
  const consultationIdRef = useRef<string | null>(null);

  useEffect(() => {
    // Initialize Consultation on mount
    createConsultation();

    // Initialize Web Speech API
    const { webkitSpeechRecognition, SpeechRecognition } = window as unknown as IWindow;
    const SpeechRecognitionConstructor = SpeechRecognition || webkitSpeechRecognition;

    if (SpeechRecognitionConstructor) {
      recognitionRef.current = new SpeechRecognitionConstructor();
      recognitionRef.current.continuous = false;
      recognitionRef.current.lang = 'ru-RU';

      recognitionRef.current.onresult = (event: any) => {
        const transcript = event.results[0][0].transcript;
        handleUserMessage(transcript);
      };

      recognitionRef.current.onend = () => setIsListening(false);
    }
  }, []);

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

  const handleUserMessage = async (text: string) => {
    setMessages(prev => [...prev, { role: 'user', text }]);

    if (!consultationIdRef.current) return;

    try {
      const res = await fetch('/api/consultation/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
            consultation_id: consultationIdRef.current, 
            text: text 
        }),
      });
      
      const data = await res.json();
      const aiResponse = data.response;

      setMessages(prev => [...prev, { role: 'assistant', text: aiResponse }]);
      speakResponse(aiResponse);

    } catch (error) {
      console.error("Error sending message", error);
    }
  };

  const speakResponse = (text: string) => {
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = 'ru-RU';
    window.speechSynthesis.speak(utterance);
  };

  const toggleRecording = () => {
    if (isListening) {
      recognitionRef.current?.stop();
    } else {
      recognitionRef.current?.start();
      setIsListening(true);
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
          <div className="h-10 w-10 bg-white/20 rounded-full flex items-center justify-center">
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
            </svg>
          </div>
        </div>

        {/* Chat Area */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4 bg-gray-50">
          {messages.length === 0 && (
            <div className="text-center text-gray-400 mt-20">
              <p className="text-lg">Нажмите кнопку микрофона, чтобы начать общение</p>
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
                Нажмите, чтобы говорить
              </>
            )}
          </button>
          <p className="text-center text-gray-400 text-xs mt-3">
            Ваши данные защищены и используются только для медицинского анализа
          </p>
        </div>
      </div>
    </div>
  );
};

export default VoiceChat;
