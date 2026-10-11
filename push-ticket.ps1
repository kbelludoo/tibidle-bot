param(
    [string]$Target = ""
)

if ($Target -ne "") {
    Write-Host "Obtendo ticket fresco no navegador e enviando para $Target via HTTP..." -ForegroundColor Cyan
    go run ./cmd/get_ticket -push $Target
} else {
    Write-Host "Obtendo ticket fresco no navegador e enviando para a VPS via SSH..." -ForegroundColor Cyan
    go run ./cmd/get_ticket
}
