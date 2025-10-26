import torch
from fastapi import FastAPI, HTTPException, UploadFile, File
from pydantic import BaseModel
import io
import uvicorn
from fastapi.responses import Response
import soundfile as sf
import numpy as np
from faster_whisper import WhisperModel
import os
import tempfile

app = FastAPI()

# Load TTS model on startup
device = torch.device('cpu')
local_file = 'model.pt'

print("Loading Silero TTS model (V5)...")
# Using V5 model which has better quality and stress handling
model, _ = torch.hub.load(repo_or_dir='snakers4/silero-models',
                          model='silero_tts',
                          language='ru',
                          speaker='v5_ru')
model.to(device)
print("TTS Model loaded.")

# Load Whisper STT model
print("Loading Whisper STT model...")
# "tiny" or "base" are good for CPU. "small" might be slow.
stt_model = WhisperModel("base", device="cpu", compute_type="int8")
print("STT Model loaded.")

class TTSRequest(BaseModel):
    text: str
    speaker: str = "kseniya" # Options: aidar, baya, kseniya, xenia, eugene
    sample_rate: int = 48000

@app.post("/generate")
async def generate_audio(req: TTSRequest):
    try:
        # Reverting SSML as it caused 500 errors. 
        # Using text with auto-accents for better quality.
        audio = model.apply_tts(text=req.text,
                                speaker=req.speaker,
                                sample_rate=req.sample_rate,
                                put_accent=True,
                                put_yo=True)
        
        # Convert tensor to wav bytes using soundfile directly
        # audio is a 1D tensor
        audio_np = audio.numpy()
        
        buffer = io.BytesIO()
        sf.write(buffer, audio_np, req.sample_rate, format='WAV')
        buffer.seek(0)
        
        return Response(content=buffer.read(), media_type="audio/wav")
        
    except Exception as e:
        print(f"Error generating audio: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/transcribe")
async def transcribe_audio(file: UploadFile = File(...)):
    try:
        # Save uploaded file to temp file because Whisper needs a file path
        with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as tmp:
            content = await file.read()
            tmp.write(content)
            tmp_path = tmp.name

        segments, info = stt_model.transcribe(tmp_path, beam_size=5)
        
        text = ""
        for segment in segments:
            text += segment.text + " "
            
        # Cleanup
        os.remove(tmp_path)
        
        return {"text": text.strip(), "language": info.language}

    except Exception as e:
        print(f"Error transcribing audio: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
