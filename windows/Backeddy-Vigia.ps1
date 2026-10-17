# Backeddy Server — vigia no Windows (para a máquina virtual, que não tem alto-falante).
# Fica na bandeja do relógio. A cada minuto pergunta à VM se há alerta; havendo, o Windows
# bipa e mostra um aviso. Reconhecer o alerta é no painel (clique no aviso ou no ícone).
# Instalado e iniciado pelo Instalar-Backeddy.ps1 (tarefa agendada ao entrar no Windows).
# Este arquivo precisa ficar salvo em UTF-8 com BOM (acentos no PowerShell 5).

$ErrorActionPreference = "SilentlyContinue"
Add-Type -AssemblyName System.Windows.Forms, System.Drawing
$cfgPath = Join-Path $env:ProgramData "Backeddy\vigia.json"
# idioma do Windows: português, espanhol; os outros, inglês
$Lang = switch ([Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName) { "pt" { 0 } "es" { 2 } default { 1 } }
$Txt = @{
    connecting = @("Backeddy: conectando…", "Backeddy: connecting…", "Backeddy: conectando…")
    openpanel  = @("Abrir o painel do Backeddy", "Open the Backeddy panel", "Abrir el panel de Backeddy")
    quit       = @("Sair do vigia", "Quit the watcher", "Salir del vigilante")
    noanswer   = @("Backeddy: servidor não responde", "Backeddy: server not responding", "Backeddy: el servidor no responde")
    down10     = @("O servidor de backup não responde há 10 minutos. A máquina virtual está ligada?", "The backup server hasn't answered for 10 minutes. Is the virtual machine on?", "El servidor de backup no responde desde hace 10 minutos. ¿La máquina virtual está encendida?")
    nalerts    = @("Backeddy: {0} alerta(s)", "Backeddy: {0} alert(s)", "Backeddy: {0} alerta(s)")
    atttitle   = @("Backeddy Server: atenção", "Backeddy Server: attention", "Backeddy Server: atención")
    atttext    = @("Algo precisa da sua atenção no servidor de backup. Clique para abrir o painel e ver.", "Something on the backup server needs your attention. Click to open the panel.", "Algo en el servidor de backup necesita tu atención. Haz clic para abrir el panel.")
    allgood    = @("Backeddy: tudo certo", "Backeddy: all good", "Backeddy: todo bien")
}
function T([string]$k) { return $Txt[$k][$Lang] }
$cfg = Get-Content -Raw $cfgPath | ConvertFrom-Json
$hosts = @($cfg.host, $cfg.ip) | Where-Object { $_ }
$script:server = $hosts[0]

# um vigia só por usuário
$mutex = New-Object Threading.Mutex($false, "Local\BackeddyVigia")
if (-not $mutex.WaitOne(0)) { exit }

$icon = New-Object Windows.Forms.NotifyIcon
$icon.Icon = [Drawing.SystemIcons]::Shield
$icon.Text = (T "connecting")
$icon.Visible = $true

function Open-Panel { Start-Process "http://$script:server/" }
$menu = New-Object Windows.Forms.ContextMenuStrip
[void]$menu.Items.Add((T "openpanel"), $null, { Open-Panel })
[void]$menu.Items.Add((T "quit"), $null, { $icon.Visible = $false; [Windows.Forms.Application]::Exit() })
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
        $icon.Text = (T "noanswer")
        # 10 minutos sem resposta: a própria VM pode estar desligada
        if ($script:fails -eq 10) {
            $icon.ShowBalloonTip(15000, "Backeddy Server", (T "down10"), "Warning")
        }
        return
    }
    $script:fails = 0
    if ($resp.alerts -gt 0) {
        $icon.Icon = [Drawing.SystemIcons]::Warning
        $icon.Text = (T "nalerts") -f $resp.alerts
        if ($resp.beep) {
            for ($i = 0; $i -lt 3; $i++) { [Console]::Beep(1760, 180); Start-Sleep -Milliseconds 120 }
            if (((Get-Date) - $script:lastBalloon).TotalMinutes -ge 10) {
                $icon.ShowBalloonTip(15000, (T "atttitle"), (T "atttext"), "Warning")
                $script:lastBalloon = Get-Date
            }
        }
    } else {
        $icon.Icon = [Drawing.SystemIcons]::Shield
        $icon.Text = (T "allgood")
    }
}

$timer = New-Object Windows.Forms.Timer
$timer.Interval = 60000
$timer.Add_Tick({ Tick })
$timer.Start()
Tick
[Windows.Forms.Application]::Run()
$icon.Dispose()
