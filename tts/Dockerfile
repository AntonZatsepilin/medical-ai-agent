FROM python:3.10-slim

WORKDIR /app

# Install system dependencies for audio
RUN apt-get update && apt-get install -y \
    libsndfile1 \
    ffmpeg \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
# Allow pip to find the best wheels for the architecture (including ARM)
RUN pip install --no-cache-dir torch torchaudio
RUN pip install --no-cache-dir fastapi uvicorn omegaconf soundfile numpy scipy faster-whisper python-multipart

COPY server.py .

# Pre-download the model during build to speed up startup
# Commented out to speed up build. Models will be downloaded on first run.
# RUN python -c "import torch; torch.hub.load(repo_or_dir='snakers4/silero-models', model='silero_tts', language='ru', speaker='v5_ru')"
# RUN python -c "from faster_whisper import WhisperModel; WhisperModel('medium', device='cpu', compute_type='int8')"

CMD ["uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8000"]
