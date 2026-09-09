# 在 Jia_api 目录下执行：初始化/升级数据库 schema（默认写入 data\jia.db）。
Set-Location (Split-Path -Parent $MyInvocation.MyCommand.Path)
Write-Host "工作目录: $(Get-Location)"
go run ./cmd/migrate
