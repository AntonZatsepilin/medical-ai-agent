import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import io
import uvicorn
from fastapi.responses import Response

app = FastAPI()

# Load model on startup
device = torch.device('cpu')
local_file = 'model.pt'

print("Loading Silero TTS model...")
model, _ = torch.hub.load(repo_or_dir='snakers4/silero-models',
                          model='silero_tts',
                          language='ru',
                          speaker='v4_ru')
model.to(device)
print("Model loaded.")

class TTSRequest(BaseModel):
    text: str
    speaker: str = "xenia" # Options: aidar, baya, kseniya, xenia, eugene
    sample_rate: int = 48000

@app.post("/generate")
async def generate_audio(req: TTSRequest):
    try:
        audio = model.apply_tts(text=req.text,
                                speaker=req.speaker,
                                sample_rate=req.sample_rate)
        
        # Convert tensor to wav bytes
        # Silero returns a tensor. We need to save it to a buffer.
        # The model has a save_wav method but it saves to disk.
        # We can use torchaudio or simple wave module if we convert to numpy.
        # Actually, model.apply_tts returns a 1D tensor.
        
        # Let's use a helper to convert to WAV in memory
        # Standard way with torchaudio:
        import torchaudio
        
        # Add batch dimension [1, T]
        audio_tensor = audio.unsqueeze(0)
        
        buffer = io.BytesIO()
        torchaudio.save(buffer, audio_tensor, req.sample_rate, format="wav")
        buffer.seek(0)
        
        return Response(content=buffer.read(), media_type="audio/wav")
        
    except Exception as e:
        print(f"Error generating audio: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
