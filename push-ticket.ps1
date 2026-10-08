param(
    [string]$Target = "http://129.153.157.17:3001"
)

Write-Host "Obtendo ticket fresco no navegador e enviando para $Target..." -ForegroundColor Cyan
go run ./cmd/get_ticket -push $Target
