@echo off
rem Abre o instalador do Backeddy Server (pede permissao de administrador).
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0Instalar-Backeddy.ps1" %*
