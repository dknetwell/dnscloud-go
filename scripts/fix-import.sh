#!/bin/bash

echo "Fixing imports in Go files..."

# Для всех .go файлов заменяем полные пути на относительные
find . -name "*.go" -type f ! -path "./vendor/*" | while read file; do
    echo "Processing $file"
    
    # Заменяем github.com/dknetwell/dnscloud-go/ на пустую строку
    sed -i 's|"github.com/dknetwell/dnscloud-go/|"|g' "$file"
    
    # Заменяем github.com/proxy/dnscloud-go/ на пустую строку  
    sed -i 's|"github.com/proxy/dnscloud-go/|"|g' "$file"
done

echo "Done!"
