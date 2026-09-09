# 从项目根目录启动 Jia API，保证数据路径 data\jia.db 与存储目录 storage 相对项目根目录解析。
Set-Location (Split-Path -Parent $MyInvocation.MyCommand.Path)
Write-Host "工作目录: $(Get-Location)"
go run ./cmd/server
