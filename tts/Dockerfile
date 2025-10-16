FROM python:3.10-slim

WORKDIR /app

# Install system dependencies for audio
RUN apt-get update && apt-get install -y \
    libsndfile1 \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
# torch and torchaudio are large, so we install them first to cache the layer
RUN pip install --no-cache-dir torch torchaudio --index-url https://download.pytorch.org/whl/cpu
RUN pip install --no-cache-dir fastapi uvicorn omegaconf soundfile numpy

COPY server.py .

# Pre-download the model during build to speed up startup
RUN python -c "import torch; torch.hub.load(repo_or_dir='snakers4/silero-models', model='silero_tts', language='ru', speaker='v5_ru')"

CMD ["uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8000"]
