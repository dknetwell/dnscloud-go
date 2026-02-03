# DNS Security Proxy

Простой DNS прокси с кешированием и метриками на базе CoreDNS.

## Быстрый старт

```bash
# 1. Клонируйте репозиторий
git clone https://github.com/dknetwell/dnscloud-go.git
cd dnscloud-go

# 2. Исправьте импорты (одноразово)
chmod +x fix-imports.sh
./fix-imports.sh

# 3. Запустите скрипт настройки
chmod +x setup.sh
./setup.sh

# Скрипт попросит настроить .env файл
# Отредактируйте .env и установите ваш CLOUD_API_KEY
# Затем запустите скрипт снова:
./setup.sh
