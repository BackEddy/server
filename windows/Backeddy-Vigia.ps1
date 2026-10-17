# Backeddy Server — vigia no Windows (para a máquina virtual, que não tem alto-falante).
# Fica na bandeja do relógio. A cada minuto pergunta à VM se há alerta; havendo, o Windows
# bipa e mostra um aviso. Reconhecer o alerta é no painel (clique no aviso ou no ícone).
# Instalado e iniciado pelo Instalar-Backeddy.ps1 (tarefa agendada ao entrar no Windows).
# Este arquivo precisa ficar salvo em UTF-8 com BOM (acentos no PowerShell 5).

$ErrorActionPreference = "SilentlyContinue"
Add-Type -AssemblyName System.Windows.Forms, System.Drawing
$cfgPath = Join-Path $env:ProgramData "Backeddy\vigia.json"
$cfg = Get-Content -Raw $cfgPath | ConvertFrom-Json
$hosts = @($cfg.host, $cfg.ip) | Where-Object { $_ }
$script:server = $hosts[0]

# um vigia só por usuário
$mutex = New-Object Threading.Mutex($false, "Local\BackeddyVigia")
if (-not $mutex.WaitOne(0)) { exit }

$icon = New-Object Windows.Forms.NotifyIcon
$icon.Icon = [Drawing.SystemIcons]::Shield
$icon.Text = "Backeddy: conectando…"
$icon.Visible = $true

function Open-Panel { Start-Process "http://$script:server/" }
$menu = New-Object Windows.Forms.ContextMenuStrip
[void]$menu.Items.Add("Abrir o painel do Backeddy", $null, { Open-Panel })
[void]$menu.Items.Add("Sair do vigia", $null, { $icon.Visible = $false; [Windows.Forms.Application]::Exit() })
$icon.ContextMenuStrip = $menu
$icon.Add_DoubleClick({ Open-Panel })
$icon.Add_BalloonTipClicked({ Open-Panel })

$script:lastBalloon = [datetime]::MinValue
$script:fails = 0

function Tick {
    $resp = $null
    foreach ($h in $hosts) {
        try { $resp = Invoke-RestMethod -UseBasicParsing -TimeoutSec 5 "http://$h/api/beep"; $script:server = $h; break } catch {}
    }
    if ($null -eq $resp) {
        $script:fails++
        $icon.Text = "Backeddy: servidor não responde"
        # 10 minutos sem resposta: a própria VM pode estar desligada
        if ($script:fails -eq 10) {
            $icon.ShowBalloonTip(15000, "Backeddy Server", "O servidor de backup não responde há 10 minutos. A máquina virtual está ligada?", "Warning")
        }
        return
    }
    $script:fails = 0
    if ($resp.alerts -gt 0) {
        $icon.Icon = [Drawing.SystemIcons]::Warning
        $icon.Text = "Backeddy: $($resp.alerts) alerta(s)"
        if ($resp.beep) {
            for ($i = 0; $i -lt 3; $i++) { [Console]::Beep(1760, 180); Start-Sleep -Milliseconds 120 }
            if (((Get-Date) - $script:lastBalloon).TotalMinutes -ge 10) {
                $icon.ShowBalloonTip(15000, "Backeddy Server: atenção", "Algo precisa da sua atenção no servidor de backup. Clique para abrir o painel e ver.", "Warning")
                $script:lastBalloon = Get-Date
            }
        }
    } else {
        $icon.Icon = [Drawing.SystemIcons]::Shield
        $icon.Text = "Backeddy: tudo certo"
    }
}

$timer = New-Object Windows.Forms.Timer
$timer.Interval = 60000
$timer.Add_Tick({ Tick })
$timer.Start()
Tick
[Windows.Forms.Application]::Run()
$icon.Dispose()
