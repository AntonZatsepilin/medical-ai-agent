import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import io
import uvicorn
from fastapi.responses import Response
import soundfile as sf
import numpy as np

app = FastAPI()

# Load model on startup
device = torch.device('cpu')
local_file = 'model.pt'

print("Loading Silero TTS model (V5)...")
# Using V5 model which has better quality and stress handling
model, _ = torch.hub.load(repo_or_dir='snakers4/silero-models',
                          model='silero_tts',
                          language='ru',
                          speaker='v5_ru')
model.to(device)
print("Model loaded.")

class TTSRequest(BaseModel):
    text: str
    speaker: str = "aidar" # Options: aidar, baya, kseniya, xenia, eugene
    sample_rate: int = 48000

@app.post("/generate")
async def generate_audio(req: TTSRequest):
    try:
        audio = model.apply_tts(text=req.text,
                                speaker=req.speaker,
                                sample_rate=req.sample_rate)
        
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

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
