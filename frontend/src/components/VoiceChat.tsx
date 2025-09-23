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
      recognitionRef.current.lang = 'en-US';

      recognitionRef.current.onresult = (event: any) => {
        const transcript = event.results[0][0].transcript;
        handleUserMessage(transcript);
      };

      recognitionRef.current.onend = () => setIsListening(false);
    }
  }, []);

  const createConsultation = async () => {
    try {
      const res = await fetch('http://localhost:8080/api/consultation', {
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
      const res = await fetch('http://localhost:8080/api/consultation/chat', {
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
    <div className="p-4 max-w-md mx-auto bg-white rounded-xl shadow-md">
      <h2 className="text-xl font-bold mb-4">AI Patient Assistant</h2>
      <div className="h-96 overflow-y-auto mb-4 border p-2 rounded bg-gray-50">
        {messages.map((m, i) => (
          <div key={i} className={`p-2 my-1 rounded max-w-[80%] ${m.role === 'user' ? 'bg-blue-100 ml-auto text-right' : 'bg-green-100 mr-auto'}`}>
            <p className="text-sm text-gray-600 mb-1">{m.role === 'user' ? 'You' : 'Assistant'}</p>
            {m.text}
          </div>
        ))}
      </div>
      
      <button 
        onClick={toggleRecording}
        className={`w-full p-3 rounded-full font-bold text-white transition-colors ${
          isListening ? 'bg-red-500 animate-pulse' : 'bg-blue-600 hover:bg-blue-700'
        }`}
      >
        {isListening ? 'Stop Listening' : 'Tap to Speak'}
      </button>
    </div>
  );
};

export default VoiceChat;
