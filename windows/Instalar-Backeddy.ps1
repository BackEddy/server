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

function Msg([string]$texto, [string]$titulo = "Backeddy Server", [string]$icone = "Information", [string]$botoes = "OK") {
    return [System.Windows.Forms.MessageBox]::Show($texto, $titulo, $botoes, $icone)
}

function Falha([string]$texto) {
    Log "ERRO: $texto"
    Msg ($texto + "`n`nDetalhes em: $Log") "Backeddy Server" "Error" | Out-Null
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
    Falha ("A virtualização do processador está desligada na BIOS/UEFI deste PC.`n`n" +
        "Reinicie, entre na BIOS e ligue a opção:`n" +
        " • Intel: ""Intel Virtualization Technology"" (VT-x)`n" +
        " • AMD: ""SVM Mode"" (AMD-V)`n`nDepois rode este instalador de novo.")
}

$Motor = ""
if ($hv -eq "Enabled") {
    $Motor = "hyperv"
} elseif ($hv -eq "Disabled") {
    $r = Msg ("Este Windows tem o Hyper-V, o jeito mais leve de rodar o Backeddy, mas ele está desligado.`n`n" +
        "Sim: ligar o Hyper-V (o PC reinicia uma vez e a instalação continua sozinha)`n" +
        "Não: usar o VirtualBox, sem reiniciar") "Backeddy Server" "Question" "YesNoCancel"
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
            Msg "O Hyper-V foi ligado. O PC vai reiniciar agora e, depois de entrar no Windows, a instalação continua sozinha." | Out-Null
            Restart-Computer -Force
            exit
        } catch {
            Log "não deu para ligar o Hyper-V: $_"
            Msg "Não deu para ligar o Hyper-V ($($_.Exception.Message)). Vamos usar o VirtualBox." "Backeddy Server" "Warning" | Out-Null
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
        Falha "Já existe uma máquina virtual ""$VmName"" neste PC. Para reinstalar, apague-a antes no Gerenciador do Hyper-V."
    }
} else {
    if ($cs.HypervisorPresent) {
        Msg ("O Hyper-V (ou WSL/Docker/Isolamento de núcleo) está ativo neste PC: o VirtualBox vai funcionar, mas mais devagar.") "Backeddy Server" "Warning" | Out-Null
    }
    if (-not (Test-Path $VBox)) {
        $r = Msg "Para rodar o Backeddy, vou baixar e instalar o VirtualBox (gratuito, código aberto). Continuar?" "Backeddy Server" "Question" "YesNo"
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
        } catch { Falha "Não deu para instalar o VirtualBox: $($_.Exception.Message)" }
        if (-not (Test-Path $VBox)) { Falha "O VirtualBox não foi instalado." }
    }
    if ((& $VBox list vms) -match [regex]::Escape("""$VmName""")) {
        Falha "Já existe uma máquina virtual ""$VmName"" no VirtualBox. Para reinstalar, apague-a antes no VirtualBox."
    }
}
Log "motor: $Motor"

# ---------- 2. Onde e de que tamanho (uma janela só) ----------
function Show-Form {
    $f = New-Object Windows.Forms.Form
    $f.Text = "Instalar o Backeddy Server"; $f.Size = New-Object Drawing.Size(520, 330)
    $f.StartPosition = "CenterScreen"; $f.FormBorderStyle = "FixedDialog"; $f.MaximizeBox = $false
    $f.Font = New-Object Drawing.Font("Segoe UI", 10)
    function Label($t, $yy, $h = 22) { $l = New-Object Windows.Forms.Label; $l.Text = $t; $l.Location = New-Object Drawing.Point(20, $yy); $l.Size = New-Object Drawing.Size(470, $h); $f.Controls.Add($l); $l }
    Label "O Backeddy Server vai rodar numa máquina virtual dentro deste Windows. Nada aqui é apagado. Nome e senha você escolhe na tela da máquina virtual, já já." 16 64 | Out-Null
    Label "Unidade onde ficam os backups:" 90 | Out-Null
    $cDrive = New-Object Windows.Forms.ComboBox; $cDrive.DropDownStyle = "DropDownList"; $cDrive.Location = New-Object Drawing.Point(20, 114); $cDrive.Width = 460; $f.Controls.Add($cDrive)
    $drives = @(Get-PSDrive -PSProvider FileSystem | Where-Object { $_.Free -gt 70GB } | Sort-Object Free -Descending)
    foreach ($d in $drives) { [void]$cDrive.Items.Add(("{0}:  {1:N0} GB livres" -f $d.Name, ($d.Free / 1GB))) }
    if ($cDrive.Items.Count -gt 0) { $cDrive.SelectedIndex = 0 }
    Label "Tamanho do disco de backup (GB):" 154 | Out-Null
    $nSize = New-Object Windows.Forms.NumericUpDown; $nSize.Location = New-Object Drawing.Point(20, 178); $nSize.Width = 120
    $nSize.Minimum = 40; $nSize.Maximum = 64000; $f.Controls.Add($nSize)
    $hint = Label "" 180 40; $hint.Location = New-Object Drawing.Point(150, 180); $hint.Size = New-Object Drawing.Size(330, 40)
    $upd = {
        if ($cDrive.SelectedIndex -ge 0) {
            $d = $drives[$cDrive.SelectedIndex]; $free = [math]::Floor($d.Free / 1GB) - 25
            $rec = [math]::Max(40, [math]::Floor($free * 0.8))
            $nSize.Maximum = [math]::Max(40, $free); $nSize.Value = [math]::Min($rec, $nSize.Maximum)
            $hint.Text = "Recomendado: $rec GB. O arquivo cresce aos poucos, conforme os backups."
        }
    }
    $cDrive.Add_SelectedIndexChanged($upd); & $upd
    $ok = New-Object Windows.Forms.Button; $ok.Text = "Continuar"; $ok.Location = New-Object Drawing.Point(290, 240); $ok.Size = New-Object Drawing.Size(95, 32)
    $cancel = New-Object Windows.Forms.Button; $cancel.Text = "Cancelar"; $cancel.Location = New-Object Drawing.Point(390, 240); $cancel.Size = New-Object Drawing.Size(95, 32)
    $cancel.DialogResult = "Cancel"; $f.CancelButton = $cancel; $f.AcceptButton = $ok
    $f.Controls.Add($ok); $f.Controls.Add($cancel)
    $ok.Add_Click({
        if ($cDrive.SelectedIndex -lt 0) { Msg "Nenhuma unidade com pelo menos 70 GB livres." | Out-Null; return }
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
$wt.Font = New-Object Drawing.Font("Segoe UI", 12, [Drawing.FontStyle]::Bold); $wt.Text = "Instalando o Backeddy Server"
$w.Controls.Add($wt)
$StepNames = @("Baixando o pendrive", "Preparando o pendrive", "Criando os discos", "Criando e ligando a máquina virtual")
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
            $wd.Text = "$(MB $done) de $(MB $total) MB"; Pump
        }
    } finally { $in.Close(); $out.Close() }
}

# ---------- 3. Pendrive virtual (a imagem do instalador) ----------
$Ext = if ($Motor -eq "hyperv") { "vhdx" } else { "vdi" }
$Usb = Join-Path $Dir "pendrive.$Ext"
Step 0 "Procurando a versão mais recente…"
if (-not $Imagem) {
    try {
        $rel = Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/BackEddy/server/releases/latest"
        $asset = $rel.assets | Where-Object { $_.name -match "^backeddy-server-[\d.]+\.$Ext$" } | Select-Object -First 1
        if ($null -eq $asset) { throw "a versão $($rel.tag_name) não tem o pendrive em .$Ext" }
        $dl = Join-Path $Dir $asset.name
        Log "baixando $($asset.browser_download_url)"
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $wc = New-Object Net.WebClient
        $task = $wc.DownloadFileTaskAsync($asset.browser_download_url, $dl)
        while (-not $task.IsCompleted) {
            $got = if (Test-Path $dl) { (Get-Item $dl).Length } else { 0 }
            $pct = if ($asset.size) { [math]::Floor(100 * $got / $asset.size) } else { 0 }
            $wd.Text = "$($rel.tag_name): $(MB $got) de $(MB $asset.size) MB ($pct%)"; Pump
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
        $ofd.Title = "Não deu para baixar. Escolha o arquivo backeddy-server-X.$Ext (o pendrive)"
        $ofd.Filter = "Pendrive do Backeddy (*.$Ext)|*.$Ext|Todos (*.*)|*.*"
        if ($ofd.ShowDialog() -ne "OK") { exit }
        $Imagem = $ofd.FileName
    }
}
if ($Imagem) {
    if (-not (Test-Path $Imagem)) { Falha "Imagem não encontrada: $Imagem" }
    Step 1
    Copy-WithProgress $Imagem $Usb      # cópia: a original fica intacta para reinstalar
}
Log "pendrive: $Usb"

# ---------- 4. Discos: sistema (24 GB) e backups ----------
Step 2 "Sistema: 24 GB · Backups: $($cfg.SizeGB) GB (ocupam espaço aos poucos)"
$Sys = Join-Path $Dir "sistema.$Ext"
$Data = Join-Path $Dir "backups.$Ext"
foreach ($pair in @(@($Sys, 24), @($Data, $cfg.SizeGB))) {
    if (Test-Path $pair[0]) { Remove-Item -Force $pair[0] }
    if ($Motor -eq "hyperv") { New-VHD -Path $pair[0] -SizeBytes ([int64]$pair[1] * 1GB) -Dynamic | Out-Null }
    else { VBM createmedium disk --filename $pair[0] --size ([int64]$pair[1] * 1024) --format VDI --variant Standard | Out-Null }
}

# ---------- 5. Máquina virtual ----------
Step 3
if ($Motor -eq "hyperv") {
    $sw = Get-VMSwitch -SwitchType External -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $sw) {
        $route = Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1
        $nic = Get-NetAdapter -InterfaceIndex $route.ifIndex
        Msg ("Vou criar uma rede virtual em ponte na placa ""$($nic.Name)"" (o Mac precisa ver o servidor na rede).`nA internet deste PC pode cair por alguns segundos.") | Out-Null
        $sw = New-VMSwitch -Name "Backeddy (ponte)" -NetAdapterName $nic.Name -AllowManagementOS $true
        Log "switch criado: $($sw.Name) em $($nic.Name)"
    }
    New-VM -Name $VmName -Generation 2 -MemoryStartupBytes 512MB -SwitchName $sw.Name -Path $Dir -NoVHD | Out-Null
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
    VBM modifyvm $VmName --memory 512 --cpus 2 --firmware efi --nic1 bridged --bridgeadapter1 $bridge `
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
$wt.Text = "Agora, na tela da máquina virtual"
$wl.Text = "1. Faça a instalação na janela da máquina virtual (nome, senha, discos).`n`n" +
    "2. Quando aparecer ""Pronto! Retire o pendrive"", volte aqui e clique em Ejetar pendrive. A máquina virtual reinicia sozinha.`n`n" +
    "Esta janela fica por cima das outras para você não perder."
$wl.Visible = $true
$be = New-Object Windows.Forms.Button; $be.Text = "Ejetar pendrive"; $be.Location = New-Object Drawing.Point(235, 200); $be.Size = New-Object Drawing.Size(140, 34)
$bc = New-Object Windows.Forms.Button; $bc.Text = "Abrir a tela da VM"; $bc.Location = New-Object Drawing.Point(20, 200); $bc.Size = New-Object Drawing.Size(160, 34)
$bx = New-Object Windows.Forms.Button; $bx.Text = "Cancelar"; $bx.Location = New-Object Drawing.Point(385, 200); $bx.Size = New-Object Drawing.Size(95, 34)
$w.Controls.AddRange(@($be, $bc, $bx))
$be.Add_Click({
    $r = Msg "A tela da máquina virtual já mostra ""Pronto!""?" "Backeddy Server" "Question" "YesNo"
    if ($r -eq "Yes") { try { Eject-Usb; $w.Tag = "ok" } catch { Msg "Não deu para ejetar: $($_.Exception.Message)" | Out-Null } }
})
$bc.Add_Click({ if ($Motor -eq "hyperv") { Start-Process vmconnect.exe -ArgumentList "localhost", "`"$VmName`"" } else { & $VBox startvm $VmName --type separate 2>$null } })
$bx.Add_Click({ $w.Tag = "cancel" })
$w.TopMost = $true; $w.Activate()
while (-not $w.Tag) { Pump; Start-Sleep -Milliseconds 50 }
if ($w.Tag -ne "ok") {
    $w.Close()
    Msg "Instalação interrompida. A máquina virtual ""$VmName"" continua criada; para recomeçar, apague-a e rode este instalador de novo." | Out-Null
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
$wt.Text = "Preparando o servidor"
$wl2 = $wl; $wl2.Text = "Preparando o servidor (uns 5 minutos)…"; Pump
$ip = $null; $hello = $null; $t0 = Get-Date
while (((Get-Date) - $t0).TotalMinutes -lt 20) {
    $min = [math]::Floor(((Get-Date) - $t0).TotalMinutes)
    if (-not $ip) { $ip = Find-VmIp }
    if ($ip) {
        $wl2.Text = "Preparando o servidor (uns 5 minutos)…`nEndereço: $ip · $min min"
        try { $hello = Invoke-RestMethod -UseBasicParsing -TimeoutSec 3 "http://$ip/api/hello"; if ($hello.slug) { break } } catch {}
    } else { $wl2.Text = "Preparando o servidor (uns 5 minutos)… · $min min" }
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
    Msg ("Pronto! ""$($hello.name)"" está funcionando.`n`n" +
        "Mac: Ajustes do Sistema › Geral › Time Machine › ""$($hello.name)"" (usuário: backeddy)`n" +
        "Painel: http://$($hello.slug).local (ou http://$ip)`n" +
        "Acesso: ssh admin@$($hello.slug).local`n`n" +
        "A máquina virtual liga sozinha junto com o Windows.`n" +
        "Se algo der errado, este Windows bipa e mostra um aviso (ícone do escudo, perto do relógio).") | Out-Null
    Start-Process "http://$ip/"
} else {
    Msg ("A máquina virtual foi instalada, mas o servidor ainda não respondeu aqui.`n" +
        "Veja a tela da máquina virtual: ela mostra o endereço quando termina.`n`nDetalhes em: $Log") "Backeddy Server" "Warning" | Out-Null
}
