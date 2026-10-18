# Backeddy Server para Windows — cria uma máquina virtual, sem formatar nada.
# Para quem já tem um PC/servidor/NAS com Windows ligado o tempo todo.
#
# A VM dá partida pelo "pendrive virtual" (a mesma imagem do pendrive de verdade): a pessoa faz a
# instalação na tela da VM e, no fim, clica em "Ejetar pendrive" aqui; a VM reinicia sozinha.
#
# Uso: clique duas vezes em "Instalar Backeddy.cmd" (ele pede permissão de administrador).
#      Opcional: -Imagem C:\caminho\backeddy-server-X.vhdx (ou .vdi); sem isso, baixa a versão mais recente
#
# Escolha automática: usa o Hyper-V (liga se preciso, com uma reinicialização) e, se não der,
# o VirtualBox (Windows Home). Este arquivo precisa ficar salvo em UTF-8 com BOM (acentos no PowerShell 5).

param(
    [string]$Imagem = "",
    [switch]$Continuar   # depois de reiniciar para ligar o Hyper-V
)

$ErrorActionPreference = "Stop"
$Base = Join-Path $env:ProgramData "Backeddy"
$Log = Join-Path $Base "instalador.log"
$VmName = "Backeddy Server"
New-Item -ItemType Directory -Force -Path $Base | Out-Null
# rodando como .exe (gerado no GitHub) ou como script .ps1
$IsExe = [Diagnostics.Process]::GetCurrentProcess().ProcessName -notmatch '^(powershell|pwsh)$'
$SelfPath = if ($IsExe) { [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName } else { $PSCommandPath }
# @@VIGIA@@  (no .exe, o GitHub troca esta linha pelo vigia embutido: $VigiaScript = @'...'@)
if (-not $VigiaScript -and -not $IsExe) {
    $vf = Join-Path $PSScriptRoot "Backeddy-Vigia.ps1"
    if (Test-Path $vf) { $VigiaScript = Get-Content -Raw -Encoding UTF8 $vf }
}

function Log([string]$m) { Add-Content -Path $Log -Value ("{0:yyyy-MM-dd HH:mm:ss} {1}" -f (Get-Date), $m) }

Add-Type -AssemblyName System.Windows.Forms, System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

# ---------- Idioma: o do Windows (português ou espanhol; os outros, inglês) ----------
$Lang = switch ([Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName) { "pt" { 0 } "es" { 2 } default { 1 } }
$Txt = @{
    bios = @("A virtualização do processador está desligada na BIOS/UEFI deste PC.`n`nReinicie, entre na BIOS e ligue a opção:`n • Intel: ""Intel Virtualization Technology"" (VT-x)`n • AMD: ""SVM Mode"" (AMD-V)`n`nDepois rode este instalador de novo.", "Processor virtualization is turned off in this PC's BIOS/UEFI.`n`nRestart, enter the BIOS and turn on:`n • Intel: ""Intel Virtualization Technology"" (VT-x)`n • AMD: ""SVM Mode"" (AMD-V)`n`nThen run this installer again.", "La virtualización del procesador está desactivada en la BIOS/UEFI de este PC.`n`nReinicia, entra en la BIOS y activa la opción:`n • Intel: ""Intel Virtualization Technology"" (VT-x)`n • AMD: ""SVM Mode"" (AMD-V)`n`nDespués vuelve a ejecutar este instalador.")
    hvoff = @("Este Windows tem o Hyper-V, o jeito mais leve de rodar o Backeddy, mas ele está desligado.`n`nSim: ligar o Hyper-V (o PC reinicia uma vez e a instalação continua sozinha)`nNão: usar o VirtualBox, sem reiniciar", "This Windows has Hyper-V, the lightest way to run Backeddy, but it's turned off.`n`nYes: turn on Hyper-V (the PC restarts once and setup continues by itself)`nNo: use VirtualBox, without restarting", "Este Windows tiene Hyper-V, la forma más ligera de ejecutar Backeddy, pero está desactivado.`n`nSí: activar Hyper-V (el PC se reinicia una vez y la instalación sigue sola)`nNo: usar VirtualBox, sin reiniciar")
    hvon = @("O Hyper-V foi ligado. O PC vai reiniciar agora e, depois de entrar no Windows, a instalação continua sozinha.", "Hyper-V has been turned on. The PC will restart now and, after you sign in to Windows, setup continues by itself.", "Hyper-V se activó. El PC se reiniciará ahora y, después de entrar en Windows, la instalación sigue sola.")
    hvfail = @("Não deu para ligar o Hyper-V ({0}). Vamos usar o VirtualBox.", "Couldn't turn on Hyper-V ({0}). We'll use VirtualBox.", "No se pudo activar Hyper-V ({0}). Usaremos VirtualBox.")
    existshv = @("Já existe uma máquina virtual ""{0}"" neste PC. Para reinstalar, apague-a antes no Gerenciador do Hyper-V.", "A virtual machine ""{0}"" already exists on this PC. To reinstall, delete it first in Hyper-V Manager.", "Ya existe una máquina virtual ""{0}"" en este PC. Para reinstalar, bórrala antes en el Administrador de Hyper-V.")
    hvslow = @("O Hyper-V (ou WSL/Docker/Isolamento de núcleo) está ativo neste PC: o VirtualBox vai funcionar, mas mais devagar.", "Hyper-V (or WSL/Docker/Core isolation) is active on this PC: VirtualBox will work, but more slowly.", "Hyper-V (o WSL/Docker/Aislamiento del núcleo) está activo en este PC: VirtualBox funcionará, pero más lento.")
    getvbox = @("Para rodar o Backeddy, vou baixar e instalar o VirtualBox (gratuito, código aberto). Continuar?", "To run Backeddy, I'll download and install VirtualBox (free, open source). Continue?", "Para ejecutar Backeddy, voy a descargar e instalar VirtualBox (gratuito, código abierto). ¿Continuar?")
    vboxfail = @("Não deu para instalar o VirtualBox: {0}", "Couldn't install VirtualBox: {0}", "No se pudo instalar VirtualBox: {0}")
    vboxmissing = @("O VirtualBox não foi instalado.", "VirtualBox was not installed.", "VirtualBox no se instaló.")
    existsvb = @("Já existe uma máquina virtual ""{0}"" no VirtualBox. Para reinstalar, apague-a antes no VirtualBox.", "A virtual machine ""{0}"" already exists in VirtualBox. To reinstall, delete it first in VirtualBox.", "Ya existe una máquina virtual ""{0}"" en VirtualBox. Para reinstalar, bórrala antes en VirtualBox.")
    formtitle = @("Instalar o Backeddy Server", "Install Backeddy Server", "Instalar Backeddy Server")
    formintro = @("O Backeddy Server vai rodar numa máquina virtual dentro deste Windows. Nada aqui é apagado. Nome e senha você escolhe na tela da máquina virtual, já já.", "Backeddy Server will run in a virtual machine inside this Windows. Nothing here gets erased. You'll pick the name and password on the virtual machine's screen in a moment.", "Backeddy Server funcionará en una máquina virtual dentro de este Windows. Aquí no se borra nada. El nombre y la contraseña los eliges en la pantalla de la máquina virtual, enseguida.")
    formdrive = @("Unidade onde ficam os backups:", "Drive where the backups will be kept:", "Unidad donde se guardarán los backups:")
    formfree = @("{0}:  {1:N0} GB livres", "{0}:  {1:N0} GB free", "{0}:  {1:N0} GB libres")
    formsize = @("Tamanho do disco de backup (GB):", "Backup disk size (GB):", "Tamaño del disco de backup (GB):")
    formhint = @("Recomendado: {0} GB. O arquivo cresce aos poucos, conforme os backups.", "Recommended: {0} GB. The file grows gradually, as backups come in.", "Recomendado: {0} GB. El archivo crece poco a poco, con los backups.")
    continue = @("Continuar", "Continue", "Continuar")
    cancel = @("Cancelar", "Cancel", "Cancelar")
    nodrive = @("Nenhuma unidade com pelo menos 70 GB livres.", "No drive with at least 70 GB free.", "Ninguna unidad con al menos 70 GB libres.")
    installing = @("Instalando o Backeddy Server", "Installing Backeddy Server", "Instalando Backeddy Server")
    step0 = @("Baixando o pendrive", "Downloading the USB drive", "Descargando el pendrive")
    step1 = @("Preparando o pendrive", "Preparing the USB drive", "Preparando el pendrive")
    step2 = @("Criando os discos", "Creating the disks", "Creando los discos")
    step3 = @("Criando e ligando a máquina virtual", "Creating and starting the virtual machine", "Creando y encendiendo la máquina virtual")
    mbof = @("{0} de {1} MB", "{0} of {1} MB", "{0} de {1} MB")
    looking = @("Procurando a versão mais recente…", "Looking for the latest version…", "Buscando la versión más reciente…")
    pickimg = @("Não deu para baixar. Escolha o arquivo backeddy-server-X.{0} (o pendrive)", "Download failed. Choose the backeddy-server-X.{0} file (the USB drive)", "No se pudo descargar. Elige el archivo backeddy-server-X.{0} (el pendrive)")
    pickfilter = @("Pendrive do Backeddy (*.{0})|*.{0}|Todos (*.*)|*.*", "Backeddy USB drive (*.{0})|*.{0}|All (*.*)|*.*", "Pendrive de Backeddy (*.{0})|*.{0}|Todos (*.*)|*.*")
    noimg = @("Imagem não encontrada: {0}", "Image not found: {0}", "Imagen no encontrada: {0}")
    disksinfo = @("Sistema: 24 GB · Backups: {0} GB (ocupam espaço aos poucos)", "System: 24 GB · Backups: {0} GB (they take up space gradually)", "Sistema: 24 GB · Backups: {0} GB (ocupan espacio poco a poco)")
    bridge = @("Vou criar uma rede virtual em ponte na placa ""{0}"" (o Mac precisa ver o servidor na rede).`nA internet deste PC pode cair por alguns segundos.", "I'll create a bridged virtual network on the ""{0}"" adapter (your Mac needs to see the server on the network).`nThis PC's internet may drop for a few seconds.", "Voy a crear una red virtual en puente en la tarjeta ""{0}"" (el Mac necesita ver el servidor en la red).`nInternet en este PC puede cortarse unos segundos.")
    nowvm = @("Agora, na tela da máquina virtual", "Now, on the virtual machine's screen", "Ahora, en la pantalla de la máquina virtual")
    nowvmtext = @("1. Faça a instalação na janela da máquina virtual (nome, senha, discos).`n`n2. Quando aparecer ""Pronto! Retire o pendrive"", volte aqui e clique em Ejetar pendrive. A máquina virtual reinicia sozinha.`n`nEsta janela fica por cima das outras para você não perder.", "1. Go through the setup in the virtual machine's window (name, password, disks).`n`n2. When it says ""Done! Remove the USB drive"", come back here and click Eject USB drive. The virtual machine restarts by itself.`n`nThis window stays on top so you don't lose it.", "1. Haz la instalación en la ventana de la máquina virtual (nombre, contraseña, discos).`n`n2. Cuando aparezca ""¡Listo! Saca el pendrive"", vuelve aquí y haz clic en Expulsar pendrive. La máquina virtual se reinicia sola.`n`nEsta ventana queda encima de las demás para que no la pierdas.")
    eject = @("Ejetar pendrive", "Eject USB drive", "Expulsar pendrive")
    openvm = @("Abrir a tela da VM", "Open the VM screen", "Abrir la pantalla de la VM")
    askdone = @("A tela da máquina virtual já mostra ""Pronto!""?", "Does the virtual machine's screen already say ""Done!""?", "¿La pantalla de la máquina virtual ya muestra ""¡Listo!""?")
    ejectfail = @("Não deu para ejetar: {0}", "Couldn't eject: {0}", "No se pudo expulsar: {0}")
    stopped = @("Instalação interrompida. A máquina virtual ""{0}"" continua criada; para recomeçar, apague-a e rode este instalador de novo.", "Setup stopped. The virtual machine ""{0}"" is still there; to start over, delete it and run this installer again.", "Instalación interrumpida. La máquina virtual ""{0}"" sigue creada; para empezar de nuevo, bórrala y vuelve a ejecutar este instalador.")
    preparing = @("Preparando o servidor", "Getting the server ready", "Preparando el servidor")
    preparingmin = @("Preparando o servidor (uns 5 minutos)…", "Getting the server ready (about 5 minutes)…", "Preparando el servidor (unos 5 minutos)…")
    address = @("Endereço: {0} · {1} min", "Address: {0} · {1} min", "Dirección: {0} · {1} min")
    done = @("Pronto! ""{0}"" está funcionando.`n`nMac: no app Backeddy for Mac, ""Configurar com o Backeddy Server"" (ou Ajustes do Sistema › Geral › Time Machine › ""{0}"", usuário: backeddy)`nPainel: http://{1}.local (ou http://{2})`nAcesso: ssh admin@{1}.local`n`nA máquina virtual liga sozinha junto com o Windows.`nSe algo der errado, este Windows bipa e mostra um aviso (ícone do escudo, perto do relógio).", "Done! ""{0}"" is up and running.`n`nMac: in the Backeddy for Mac app, ""Set up with Backeddy Server"" (or System Settings › General › Time Machine › ""{0}"", user: backeddy)`nPanel: http://{1}.local (or http://{2})`nAccess: ssh admin@{1}.local`n`nThe virtual machine starts by itself with Windows.`nIf something goes wrong, this Windows beeps and shows a notice (shield icon, near the clock).", "¡Listo! ""{0}"" está funcionando.`n`nMac: en la app Backeddy for Mac, ""Configurar con Backeddy Server"" (o Ajustes del Sistema › General › Time Machine › ""{0}"", usuario: backeddy)`nPanel: http://{1}.local (o http://{2})`nAcceso: ssh admin@{1}.local`n`nLa máquina virtual se enciende sola con Windows.`nSi algo sale mal, este Windows pita y muestra un aviso (icono del escudo, junto al reloj).")
    noanswer = @("A máquina virtual foi instalada, mas o servidor ainda não respondeu aqui.`nVeja a tela da máquina virtual: ela mostra o endereço quando termina.`n`nDetalhes em: {0}", "The virtual machine was installed, but the server hasn't answered here yet.`nCheck the virtual machine's screen: it shows the address when it's done.`n`nDetails in: {0}", "La máquina virtual se instaló, pero el servidor aún no respondió aquí.`nMira la pantalla de la máquina virtual: muestra la dirección cuando termina.`n`nDetalles en: {0}")
    details = @("Detalhes em: {0}", "Details in: {0}", "Detalles en: {0}")
}
function T([string]$k) { $v = $Txt[$k]; if ($null -eq $v) { return $k }; return $v[$Lang] }

function Msg([string]$texto, [string]$titulo = "Backeddy Server", [string]$icone = "Information", [string]$botoes = "OK") {
    return [System.Windows.Forms.MessageBox]::Show($texto, $titulo, $botoes, $icone)
}

function Falha([string]$texto) {
    Log "ERRO: $texto"
    Msg ($texto + "`n`n" + ((T "details") -f $Log)) "Backeddy Server" "Error" | Out-Null
    exit 1
}

# ---------- administrador ----------
$id = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $id.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    if ($IsExe) {
        $a = @(); if ($Imagem) { $a += @("-Imagem", "`"$Imagem`"") }
        if ($a.Count) { Start-Process $SelfPath -Verb RunAs -ArgumentList $a } else { Start-Process $SelfPath -Verb RunAs }
    } else {
        $a = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "`"$PSCommandPath`"")
        if ($Imagem) { $a += @("-Imagem", "`"$Imagem`"") }
        Start-Process powershell -Verb RunAs -ArgumentList $a
    }
    exit
}
Log "== início (Continuar=$Continuar)"

# ---------- 1. Onde a VM vai rodar ----------
function Get-HyperVState {
    try {
        $f = Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -ErrorAction Stop
        if ($null -eq $f) { return "ausente" }
        return [string]$f.State   # Enabled / Disabled
    } catch { return "ausente" }  # Windows Home
}

$cs = Get-CimInstance Win32_ComputerSystem
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$hv = Get-HyperVState
Log "Hyper-V: $hv; hipervisor ativo: $($cs.HypervisorPresent); virtualização na BIOS: $($cpu.VirtualizationFirmwareEnabled)"

if (-not $cs.HypervisorPresent -and -not $cpu.VirtualizationFirmwareEnabled) {
    Falha (T "bios")
}

$Motor = ""
if ($hv -eq "Enabled") {
    $Motor = "hyperv"
} elseif ($hv -eq "Disabled") {
    $r = Msg (T "hvoff") "Backeddy Server" "Question" "YesNoCancel"
    if ($r -eq "Cancel") { exit }
    if ($r -eq "Yes") {
        try {
            if ($IsExe) {
                $self = Join-Path $Base "Instalar-Backeddy.exe"
                Copy-Item -Force $SelfPath $self
                $cmd = "`"$self`" -Continuar"
            } else {
                $self = Join-Path $Base "Instalar-Backeddy.ps1"
                Copy-Item -Force $PSCommandPath $self
                Copy-Item -Force (Join-Path $PSScriptRoot "Backeddy-Vigia.ps1") $Base -ErrorAction SilentlyContinue
                $cmd = "powershell -NoProfile -ExecutionPolicy Bypass -File `"$self`" -Continuar"
            }
            if ($Imagem) { $cmd += " -Imagem `"$Imagem`"" }
            Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All -NoRestart | Out-Null
            Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-Management-PowerShell -All -NoRestart -ErrorAction SilentlyContinue | Out-Null
            Set-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\RunOnce" -Name "BackeddyInstalador" -Value $cmd
            Log "Hyper-V ligado; reiniciando"
            Msg (T "hvon") | Out-Null
            Restart-Computer -Force
            exit
        } catch {
            Log "não deu para ligar o Hyper-V: $_"
            Msg ((T "hvfail") -f $_.Exception.Message) "Backeddy Server" "Warning" | Out-Null
            $Motor = "virtualbox"
        }
    } else { $Motor = "virtualbox" }
} else {
    $Motor = "virtualbox"
}
$VBox = Join-Path $env:ProgramFiles "Oracle\VirtualBox\VBoxManage.exe"
function VBM { param([Parameter(ValueFromRemainingArguments = $true)] [string[]]$a)
    $out = & $VBox @a 2>&1
    if ($LASTEXITCODE -ne 0) { throw "VBoxManage $($a -join ' '): $out" }
    return $out
}
if ($Motor -eq "hyperv") {
    try { Import-Module Hyper-V -ErrorAction Stop } catch {
        # Hyper-V ligado sem as ferramentas de PowerShell (comum no Windows Server)
        Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-Management-PowerShell -All -NoRestart | Out-Null
        Import-Module Hyper-V
    }
    if (Get-VM -Name $VmName -ErrorAction SilentlyContinue) {
        Falha ((T "existshv") -f $VmName)
    }
} else {
    if ($cs.HypervisorPresent) {
        Msg (T "hvslow") "Backeddy Server" "Warning" | Out-Null
    }
    if (-not (Test-Path $VBox)) {
        $r = Msg (T "getvbox") "Backeddy Server" "Question" "YesNo"
        if ($r -ne "Yes") { exit }
        try {
            $ProgressPreference = "SilentlyContinue"
            $tmp = Join-Path $env:TEMP "backeddy-vbox"; New-Item -ItemType Directory -Force -Path $tmp | Out-Null
            # o VirtualBox 7 precisa do Visual C++ 2019+
            Invoke-WebRequest -UseBasicParsing "https://aka.ms/vs/17/release/vc_redist.x64.exe" -OutFile "$tmp\vc_redist.x64.exe"
            Start-Process -Wait "$tmp\vc_redist.x64.exe" -ArgumentList "/install", "/quiet", "/norestart"
            $ver = (Invoke-WebRequest -UseBasicParsing "https://download.virtualbox.org/virtualbox/LATEST-STABLE.TXT").Content.Trim()
            $list = (Invoke-WebRequest -UseBasicParsing "https://download.virtualbox.org/virtualbox/$ver/").Content
            $exe = [regex]::Match($list, "VirtualBox-$([regex]::Escape($ver))-\d+-Win\.exe").Value
            if (-not $exe) { throw "não achei o instalador do VirtualBox $ver" }
            Log "baixando VirtualBox $exe"
            Invoke-WebRequest -UseBasicParsing "https://download.virtualbox.org/virtualbox/$ver/$exe" -OutFile "$tmp\$exe"
            Start-Process -Wait "$tmp\$exe" -ArgumentList "--silent", "--ignore-reboot"
        } catch { Falha ((T "vboxfail") -f $_.Exception.Message) }
        if (-not (Test-Path $VBox)) { Falha (T "vboxmissing") }
    }
    if ((& $VBox list vms) -match [regex]::Escape("""$VmName""")) {
        Falha ((T "existsvb") -f $VmName)
    }
}
Log "motor: $Motor"

# ---------- 2. Onde e de que tamanho (uma janela só) ----------
function Show-Form {
    $f = New-Object Windows.Forms.Form
    $f.Text = (T "formtitle"); $f.Size = New-Object Drawing.Size(520, 330)
    $f.StartPosition = "CenterScreen"; $f.FormBorderStyle = "FixedDialog"; $f.MaximizeBox = $false
    $f.Font = New-Object Drawing.Font("Segoe UI", 10)
    function Label($t, $yy, $h = 22) { $l = New-Object Windows.Forms.Label; $l.Text = $t; $l.Location = New-Object Drawing.Point(20, $yy); $l.Size = New-Object Drawing.Size(470, $h); $f.Controls.Add($l); $l }
    Label (T "formintro") 16 64 | Out-Null
    Label (T "formdrive") 90 | Out-Null
    $cDrive = New-Object Windows.Forms.ComboBox; $cDrive.DropDownStyle = "DropDownList"; $cDrive.Location = New-Object Drawing.Point(20, 114); $cDrive.Width = 460; $f.Controls.Add($cDrive)
    $drives = @(Get-PSDrive -PSProvider FileSystem | Where-Object { $_.Free -gt 70GB } | Sort-Object Free -Descending)
    foreach ($d in $drives) { [void]$cDrive.Items.Add(((T "formfree") -f $d.Name, ($d.Free / 1GB))) }
    if ($cDrive.Items.Count -gt 0) { $cDrive.SelectedIndex = 0 }
    Label (T "formsize") 154 | Out-Null
    $nSize = New-Object Windows.Forms.NumericUpDown; $nSize.Location = New-Object Drawing.Point(20, 178); $nSize.Width = 120
    $nSize.Minimum = 40; $nSize.Maximum = 64000; $f.Controls.Add($nSize)
    $hint = Label "" 180 40; $hint.Location = New-Object Drawing.Point(150, 180); $hint.Size = New-Object Drawing.Size(330, 40)
    $upd = {
        if ($cDrive.SelectedIndex -ge 0) {
            $d = $drives[$cDrive.SelectedIndex]; $free = [math]::Floor($d.Free / 1GB) - 25
            $rec = [math]::Max(40, [math]::Floor($free * 0.8))
            $nSize.Maximum = [math]::Max(40, $free); $nSize.Value = [math]::Min($rec, $nSize.Maximum)
            $hint.Text = (T "formhint") -f $rec
        }
    }
    $cDrive.Add_SelectedIndexChanged($upd); & $upd
    $ok = New-Object Windows.Forms.Button; $ok.Text = (T "continue"); $ok.Location = New-Object Drawing.Point(290, 240); $ok.Size = New-Object Drawing.Size(95, 32)
    $cancel = New-Object Windows.Forms.Button; $cancel.Text = (T "cancel"); $cancel.Location = New-Object Drawing.Point(390, 240); $cancel.Size = New-Object Drawing.Size(95, 32)
    $cancel.DialogResult = "Cancel"; $f.CancelButton = $cancel; $f.AcceptButton = $ok
    $f.Controls.Add($ok); $f.Controls.Add($cancel)
    $ok.Add_Click({
        if ($cDrive.SelectedIndex -lt 0) { Msg (T "nodrive") | Out-Null; return }
        $f.DialogResult = "OK"; $f.Close()
    })
    if ($f.ShowDialog() -ne "OK") { return $null }
    return [pscustomobject]@{ Drive = $drives[$cDrive.SelectedIndex].Name; SizeGB = [int]$nSize.Value }
}
$cfg = Show-Form
if ($null -eq $cfg) { exit }
$Dir = "$($cfg.Drive):\BackEddyVM"
New-Item -ItemType Directory -Force -Path $Dir | Out-Null
Log "pasta=$Dir tamanho=$($cfg.SizeGB)GB"

# ---------- Janela única: progresso → ejetar o pendrive → esperar o servidor ----------
$w = New-Object Windows.Forms.Form
$w.Text = "Backeddy Server"; $w.ClientSize = New-Object Drawing.Size(500, 250); $w.StartPosition = "CenterScreen"
$w.FormBorderStyle = "FixedDialog"; $w.MaximizeBox = $false; $w.MinimizeBox = $true; $w.ControlBox = $false
$w.Font = New-Object Drawing.Font("Segoe UI", 10)
$wt = New-Object Windows.Forms.Label; $wt.Location = New-Object Drawing.Point(20, 16); $wt.Size = New-Object Drawing.Size(460, 26)
$wt.Font = New-Object Drawing.Font("Segoe UI", 12, [Drawing.FontStyle]::Bold); $wt.Text = (T "installing")
$w.Controls.Add($wt)
$StepNames = @((T "step0"), (T "step1"), (T "step2"), (T "step3"))
$StepLabels = @()
for ($i = 0; $i -lt $StepNames.Count; $i++) {
    $l = New-Object Windows.Forms.Label; $l.Location = New-Object Drawing.Point(28, (56 + $i * 28)); $l.Size = New-Object Drawing.Size(450, 24)
    $l.Text = "○   " + $StepNames[$i]; $l.ForeColor = [Drawing.Color]::Gray
    $w.Controls.Add($l); $StepLabels += $l
}
$wd = New-Object Windows.Forms.Label; $wd.Location = New-Object Drawing.Point(28, 176); $wd.Size = New-Object Drawing.Size(450, 24)
$wd.ForeColor = [Drawing.Color]::DimGray; $w.Controls.Add($wd)
$wl = New-Object Windows.Forms.Label; $wl.Location = New-Object Drawing.Point(20, 52); $wl.Size = New-Object Drawing.Size(460, 130); $wl.Visible = $false
$w.Controls.Add($wl)
$w.Show(); [Windows.Forms.Application]::DoEvents()

function Pump { [Windows.Forms.Application]::DoEvents() }
function Step([int]$n, [string]$detalhe = "") {
    for ($i = 0; $i -lt $StepLabels.Count; $i++) {
        if ($i -lt $n) { $StepLabels[$i].Text = "✓   " + $StepNames[$i]; $StepLabels[$i].ForeColor = [Drawing.Color]::SeaGreen; $StepLabels[$i].Font = $w.Font }
        elseif ($i -eq $n) { $StepLabels[$i].Text = "›   " + $StepNames[$i] + "…"; $StepLabels[$i].ForeColor = [Drawing.Color]::Black; $StepLabels[$i].Font = New-Object Drawing.Font("Segoe UI", 10, [Drawing.FontStyle]::Bold) }
    }
    $wd.Text = $detalhe; Pump
}
function MB([int64]$b) { return [math]::Round($b / 1MB) }
# cópia em pedaços, para a janela não "congelar" com um arquivo de 1 GB
function Copy-WithProgress([string]$src, [string]$dst) {
    $in = [IO.File]::OpenRead($src); $out = [IO.File]::Create($dst)
    try {
        $buf = New-Object byte[] (8MB); $done = 0; $total = $in.Length
        while (($n = $in.Read($buf, 0, $buf.Length)) -gt 0) {
            $out.Write($buf, 0, $n); $done += $n
            $wd.Text = (T "mbof") -f (MB $done), (MB $total); Pump
        }
    } finally { $in.Close(); $out.Close() }
}

# ---------- 3. Pendrive virtual (a imagem do instalador) ----------
$Ext = if ($Motor -eq "hyperv") { "vhdx" } else { "vdi" }
$Usb = Join-Path $Dir "pendrive.$Ext"
Step 0 (T "looking")
if (-not $Imagem) {
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        # a versão mais nova que já tem o pendrive: logo depois de uma versão sair, a imagem ainda está subindo
        # (leva uns minutos), e aí vale a anterior
        $rel = $null; $asset = $null
        $rels = Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/BackEddy/server/releases?per_page=10"
        foreach ($r in $rels) {
            if ($r.draft -or $r.prerelease) { continue }
            $a = $r.assets | Where-Object { $_.name -match "^backeddy-server-[\d.]+\.$Ext$" -and $_.state -eq "uploaded" } | Select-Object -First 1
            if ($a) { $rel = $r; $asset = $a; break }
            Log "a versão $($r.tag_name) ainda não tem o pendrive em .$Ext; tentando a anterior"
        }
        if ($null -eq $asset) { throw "nenhuma versão com o pendrive em .$Ext" }
        $dl = Join-Path $Dir $asset.name
        Log "baixando $($asset.browser_download_url)"
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $wc = New-Object Net.WebClient
        $task = $wc.DownloadFileTaskAsync($asset.browser_download_url, $dl)
        while (-not $task.IsCompleted) {
            $got = if (Test-Path $dl) { (Get-Item $dl).Length } else { 0 }
            $pct = if ($asset.size) { [math]::Floor(100 * $got / $asset.size) } else { 0 }
            $wd.Text = "$($rel.tag_name): " + ((T "mbof") -f (MB $got), (MB $asset.size)) + " ($pct%)"; Pump
            Start-Sleep -Milliseconds 200
        }
        if ($task.IsFaulted) { throw $task.Exception.InnerException }
        if ((Get-Item $dl).Length -ne $asset.size) { throw "download incompleto" }
        Step 1
        Move-Item -Force $dl $Usb    # baixado só para isto: vira o pendrive direto
    } catch {
        Log "download: $_"
        if ($dl) { Remove-Item -Force $dl -ErrorAction SilentlyContinue }
        $ofd = New-Object Windows.Forms.OpenFileDialog
        $ofd.Title = (T "pickimg") -f $Ext
        $ofd.Filter = (T "pickfilter") -f $Ext
        if ($ofd.ShowDialog() -ne "OK") { exit }
        $Imagem = $ofd.FileName
    }
}
if ($Imagem) {
    if (-not (Test-Path $Imagem)) { Falha ((T "noimg") -f $Imagem) }
    Step 1
    Copy-WithProgress $Imagem $Usb      # cópia: a original fica intacta para reinstalar
}
Log "pendrive: $Usb"

# ---------- 4. Discos: sistema (24 GB) e backups ----------
Step 2 ((T "disksinfo") -f $cfg.SizeGB)
$Sys = Join-Path $Dir "sistema.$Ext"
$Data = Join-Path $Dir "backups.$Ext"
foreach ($pair in @(@($Sys, 24), @($Data, $cfg.SizeGB))) {
    if (Test-Path $pair[0]) { Remove-Item -Force $pair[0] }
    if ($Motor -eq "hyperv") { New-VHD -Path $pair[0] -SizeBytes ([int64]$pair[1] * 1GB) -Dynamic | Out-Null }
    else { VBM createmedium disk --filename $pair[0] --size ([int64]$pair[1] * 1024) --format VDI --variant Standard | Out-Null }
}

# ---------- 5. Máquina virtual ----------
# 2 GB de memória (o Kopia recebendo o primeiro backup de um PC passa de 300 MB); 1 GB em PCs com até 6 GB
$HostRam = (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory
$VmRam = if ($HostRam -le 6GB) { 1GB } else { 2GB }
Log "memória da VM: $([math]::Round($VmRam / 1GB)) GB (PC com $([math]::Round($HostRam / 1GB)) GB)"
Step 3
if ($Motor -eq "hyperv") {
    $sw = Get-VMSwitch -SwitchType External -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $sw) {
        $route = Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1
        $nic = Get-NetAdapter -InterfaceIndex $route.ifIndex
        Msg ((T "bridge") -f $nic.Name) | Out-Null
        $sw = New-VMSwitch -Name "Backeddy (ponte)" -NetAdapterName $nic.Name -AllowManagementOS $true
        Log "switch criado: $($sw.Name) em $($nic.Name)"
    }
    New-VM -Name $VmName -Generation 2 -MemoryStartupBytes $VmRam -SwitchName $sw.Name -Path $Dir -NoVHD | Out-Null
    Set-VMMemory $VmName -DynamicMemoryEnabled $false
    Set-VMProcessor $VmName -Count 2
    Set-VMFirmware $VmName -SecureBootTemplate MicrosoftUEFICertificateAuthority
    Set-VM $VmName -AutomaticCheckpointsEnabled $false -AutomaticStartAction Start -AutomaticStopAction ShutDown -AutomaticStartDelay 30
    Add-VMHardDiskDrive $VmName -Path $Usb
    Add-VMHardDiskDrive $VmName -Path $Sys
    Add-VMHardDiskDrive $VmName -Path $Data
    Set-VMFirmware $VmName -FirstBootDevice (Get-VMHardDiskDrive $VmName | Where-Object Path -eq $Usb)
    Start-VM $VmName
    Start-Process vmconnect.exe -ArgumentList "localhost", "`"$VmName`""
    $vmMac = ((Get-VMNetworkAdapter -VMName $VmName).MacAddress) -replace '(..)(?!$)', '$1-'
} else {
    $route = Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1
    $nic = Get-NetAdapter -InterfaceIndex $route.ifIndex
    $bridge = $nic.InterfaceDescription
    $names = (& $VBox list bridgedifs) | Where-Object { $_ -match '^Name:\s+(.+)$' } | ForEach-Object { $Matches[1].Trim() }
    if ($names -notcontains $bridge -and $names.Count -gt 0) { $bridge = $names | Where-Object { $_ -like "*$($nic.Name)*" } | Select-Object -First 1; if (-not $bridge) { $bridge = $names[0] } }
    VBM createvm --name $VmName --ostype Debian_64 --register --basefolder $Dir | Out-Null
    VBM modifyvm $VmName --memory ([int]($VmRam / 1MB)) --cpus 2 --firmware efi --nic1 bridged --bridgeadapter1 $bridge `
        --graphicscontroller vmsvga --vram 16 --audio-enabled off --usbohci on --mouse usbtablet | Out-Null
    VBM storagectl $VmName --name SATA --add sata --controller IntelAhci --portcount 3 --hostiocache on | Out-Null
    # pendrive na porta 0 (dá a partida) e removível com a VM ligada
    VBM storageattach $VmName --storagectl SATA --port 0 --device 0 --type hdd --medium $Usb --hotpluggable on | Out-Null
    VBM storageattach $VmName --storagectl SATA --port 1 --device 0 --type hdd --medium $Sys | Out-Null
    VBM storageattach $VmName --storagectl SATA --port 2 --device 0 --type hdd --medium $Data | Out-Null
    VBM startvm $VmName --type gui | Out-Null
    $act = New-ScheduledTaskAction -Execute $VBox -Argument "startvm `"$VmName`" --type headless"
    $trg = @((New-ScheduledTaskTrigger -AtStartup), (New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"))
    $pri = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType S4U -RunLevel Highest
    Register-ScheduledTask -TaskName "Backeddy Server (VirtualBox)" -Action $act -Trigger $trg -Principal $pri -Force | Out-Null
    $vmMac = ((& $VBox showvminfo $VmName --machinereadable) | Where-Object { $_ -match '^macaddress1="(.+)"' } | ForEach-Object { $Matches[1] }) -replace '(..)(?!$)', '$1-'
}
Log "VM criada e ligada pelo pendrive ($Motor, MAC $vmMac)"

# ---------- 6. Janela de acompanhamento: instalar na tela da VM e ejetar o pendrive ----------
function Eject-Usb {
    if ($Motor -eq "hyperv") { Get-VMHardDiskDrive $VmName | Where-Object Path -eq $Usb | Remove-VMHardDiskDrive }
    else { VBM storageattach $VmName --storagectl SATA --port 0 --device 0 --medium none | Out-Null }
    Log "pendrive ejetado"
}
# a mesma janela do progresso troca de conteúdo
foreach ($l in $StepLabels) { $l.Visible = $false }
$wd.Visible = $false
$wt.Text = (T "nowvm")
$wl.Text = (T "nowvmtext")
$wl.Visible = $true
$be = New-Object Windows.Forms.Button; $be.Text = (T "eject"); $be.Location = New-Object Drawing.Point(235, 200); $be.Size = New-Object Drawing.Size(140, 34)
$bc = New-Object Windows.Forms.Button; $bc.Text = (T "openvm"); $bc.Location = New-Object Drawing.Point(20, 200); $bc.Size = New-Object Drawing.Size(160, 34)
$bx = New-Object Windows.Forms.Button; $bx.Text = (T "cancel"); $bx.Location = New-Object Drawing.Point(385, 200); $bx.Size = New-Object Drawing.Size(95, 34)
$w.Controls.AddRange(@($be, $bc, $bx))
$be.Add_Click({
    $r = Msg (T "askdone") "Backeddy Server" "Question" "YesNo"
    if ($r -eq "Yes") { try { Eject-Usb; $w.Tag = "ok" } catch { Msg ((T "ejectfail") -f $_.Exception.Message) | Out-Null } }
})
$bc.Add_Click({ if ($Motor -eq "hyperv") { Start-Process vmconnect.exe -ArgumentList "localhost", "`"$VmName`"" } else { & $VBox startvm $VmName --type separate 2>$null } })
$bx.Add_Click({ $w.Tag = "cancel" })
$w.TopMost = $true; $w.Activate()
while (-not $w.Tag) { Pump; Start-Sleep -Milliseconds 50 }
if ($w.Tag -ne "ok") {
    $w.Close()
    Msg ((T "stopped") -f $VmName) | Out-Null
    exit
}

# ---------- 7. Esperar o servidor ficar pronto ----------
function Find-VmIp {
    if ($Motor -eq "hyperv") {
        $ip = (Get-VMNetworkAdapter -VMName $VmName).IPAddresses | Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } | Select-Object -First 1
        if ($ip) { return $ip }
    }
    $n = Get-NetNeighbor -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.LinkLayerAddress -eq $vmMac.ToUpper() -and $_.State -ne "Unreachable" } | Select-Object -First 1
    if ($n) { return $n.IPAddress }
    # ainda não está na tabela: "chama" a rede inteira (/24) para o Windows aprender o endereço da VM
    $myip = (Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex (Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1).ifIndex | Select-Object -First 1).IPAddress
    if ($myip) {
        $pre = $myip -replace '\.\d+$', ''
        $tasks = 1..254 | ForEach-Object { (New-Object Net.NetworkInformation.Ping).SendPingAsync("$pre.$_", 300) }
        try { [Threading.Tasks.Task]::WaitAll($tasks) } catch {}
    }
    return $null
}
foreach ($b in @($be, $bc, $bx)) { $b.Visible = $false }
$w.TopMost = $false
$wt.Text = (T "preparing")
$wl2 = $wl; $wl2.Text = (T "preparingmin"); Pump
$ip = $null; $hello = $null; $t0 = Get-Date
while (((Get-Date) - $t0).TotalMinutes -lt 20) {
    $min = [math]::Floor(((Get-Date) - $t0).TotalMinutes)
    if (-not $ip) { $ip = Find-VmIp }
    if ($ip) {
        $wl2.Text = (T "preparingmin") + "`n" + ((T "address") -f $ip, $min)
        try { $hello = Invoke-RestMethod -UseBasicParsing -TimeoutSec 3 "http://$ip/api/hello"; if ($hello.slug) { break } } catch {}
    } else { $wl2.Text = (T "preparingmin") + " · $min min" }
    [Windows.Forms.Application]::DoEvents(); Start-Sleep -Seconds 5
}
$w.Close()
$ready = $null -ne $hello -and $hello.slug
Log "pronto=$ready ip=$ip nome=$($hello.name)"

# ---------- 8. Vigia no Windows: bipa e avisa quando a VM tiver alerta (a VM não tem alto-falante) ----------
if ($ready) {
    try {
        $vdst = Join-Path $Base "Backeddy-Vigia.ps1"
        if (-not $VigiaScript -and (Test-Path $vdst)) { $VigiaScript = Get-Content -Raw -Encoding UTF8 $vdst }
        if (-not $VigiaScript) { throw "vigia não encontrado" }
        [IO.File]::WriteAllText($vdst, $VigiaScript, (New-Object Text.UTF8Encoding($true)))
        @{ host = "$($hello.slug).local"; ip = "$ip" } | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Base "vigia.json")
        # conhost --headless: sem janela nenhuma (no Windows 11, o -WindowStyle Hidden ainda mostra o Terminal vazio)
        $va = New-ScheduledTaskAction -Execute "conhost.exe" -Argument "--headless powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$vdst`""
        $vt = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
        $vp = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
        $vs = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
        Register-ScheduledTask -TaskName "Backeddy Vigia" -Action $va -Trigger $vt -Principal $vp -Settings $vs -Force | Out-Null
        Start-ScheduledTask -TaskName "Backeddy Vigia"
        Log "vigia instalado"
    } catch { Log "vigia: $_" }
    Remove-Item -Force $Usb -ErrorAction SilentlyContinue   # o pendrive virtual não é mais necessário
    Msg ((T "done") -f $hello.name, $hello.slug, $ip) | Out-Null
    Start-Process "http://$ip/"
} else {
    Msg ((T "noanswer") -f $Log) "Backeddy Server" "Warning" | Out-Null
}
