# Medical AI Agent

Эмпатичный медицинский голосовой ассистент для первичного опроса пациентов.

## Описание

Этот проект представляет собой голосового ассистента, который проводит первичный опрос пациентов в приемном отделении. Ассистент собирает жалобы, анализирует их, формирует структурированный отчет и отправляет его врачу в Telegram в формате PDF.

### Основные возможности:
- **Голосовой интерфейс:** Полностью голосовое управление (Hands-Free режим).
- **Умный опрос:** Использует LLM (DeepSeek) для ведения естественного диалога и уточнения симптомов.
- **Анализ эмоций:** Распознает тревожность пациента и адаптирует тон общения.
- **Генерация отчетов:** Автоматически создает PDF-отчет с анамнезом и рекомендациями.
- **Интеграция с Telegram:** Мгновенная отправка отчетов дежурному врачу.

## Технологический стек

- **Frontend:** React, TypeScript, Vite, TailwindCSS, Web Audio API (VAD).
- **Backend:** Go (Golang), Gin Web Framework.
- **AI/ML:**
  - **LLM:** DeepSeek API (Communicator, Analyst, Supervisor agents).
  - **STT:** Faster-Whisper (Docker service).
  - **TTS:** Silero V5 (Local Python service) / ElevenLabs.
- **Infrastructure:** Docker, Docker Compose.

## Быстрый старт

### Предварительные требования
- Docker и Docker Compose
- API Key для DeepSeek (или OpenAI совместимый)
- Telegram Bot Token и Chat ID (для получения отчетов)

### Установка и запуск

1. **Клонируйте репозиторий:**
   ```bash
   git clone https://github.com/your-username/medical-ai-agent.git
   cd medical-ai-agent
   ```

2. **Настройте переменные окружения:**
   Создайте файл `.env` в корне проекта (или используйте существующий в `backend/`):
   ```env
   DEEPSEEK_API_KEY=your_key_here
   TELEGRAM_BOT_TOKEN=your_bot_token
   TELEGRAM_CHAT_ID=your_chat_id
   POSTGRES_USER=medical_user
   POSTGRES_PASSWORD=postgres
   POSTGRES_DB=medical_ai
   ```

3. **Запустите проект через Docker Compose:**
   ```bash
   docker-compose up --build
   ```

4. **Откройте приложение:**
   Перейдите в браузере по адресу: `http://localhost:3000`.

### Использование

1. Нажмите кнопку микрофона или включите режим "Hands-Free".
2. Представьтесь и опишите свои симптомы.
3. Отвечайте на вопросы ассистента.
4. Когда ассистент соберет достаточно информации, он попрощается ("Врач скоро подойдет")./Users/anton/Desktop/GitHub/medical-ai-agent/tts
5. В этот момент в Telegram придет PDF-отчет.

## Структура проекта

```
.
├── backend/            # Go сервер (API, бизнес-логика, интеграции)
│   ├── cmd/            # Точка входа
│   ├── internal/       # Внутренняя логика (агенты, отчеты, консультации)
│   └── migrations/     # SQL миграции
├── frontend/           # React приложение
│   ├── src/components/ # Компоненты UI (VoiceChat)
│   └── ...
├── tts/                # Python сервис для синтеза речи (Silero)
└── docker-compose.yaml # Оркестрация контейнеров
```

## Разработка

Для локальной разработки без Docker:

1. **Backend:**
   ```bash
   cd backend
   go run cmd/server/main.go
   ```

2. **Frontend:**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

3. **TTS Service:**
   ```bash
   cd tts
   pip install -r requirements.txt
   python server.py
   ```

## Лицензия

MIT
