FROM python:3.12-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

COPY pyproject.toml README.md ./
COPY alembic.ini ./
COPY alembic ./alembic
COPY scripts ./scripts
COPY src ./src

RUN python -c "from pathlib import Path; p=Path('scripts/entrypoint.sh'); p.write_text(p.read_text().replace('\\r\\n','\\n').replace('\\r','\\n'))"

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir .

EXPOSE 8081

# На Windows файл может быть с CRLF, из-за чего прямой exec падает
# ("no such file or directory" из-за /bin/sh\r). Запускаем через sh.
CMD ["sh", "./scripts/entrypoint.sh"]
