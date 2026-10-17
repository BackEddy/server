# Backeddy Server para Windows — cria uma máquina virtual pronta, sem formatar nada.
# Para quem já tem um PC/servidor/NAS com Windows ligado o tempo todo.
#
# Uso: clique duas vezes em "Instalar Backeddy.cmd" (ele pede permissão de administrador).
#      Opcional: -Imagem C:\caminho\backeddy-server-X-vm.vhdx (sem isso, baixa a versão mais recente)
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
    $a = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "`"$PSCommandPath`"")
    if ($Imagem) { $a += @("-Imagem", "`"$Imagem`"") }
    Start-Process powershell -Verb RunAs -ArgumentList $a
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
            $self = Join-Path $Base "Instalar-Backeddy.ps1"
            Copy-Item -Force $PSCommandPath $self
            Copy-Item -Force (Join-Path $PSScriptRoot "Backeddy-Vigia.ps1") $Base -ErrorAction SilentlyContinue
            $cmd = "powershell -NoProfile -ExecutionPolicy Bypass -File `"$self`" -Continuar"
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

# ---------- 2. Perguntas (uma janela só) ----------
function Show-Form {
    $f = New-Object Windows.Forms.Form
    $f.Text = "Instalar o Backeddy Server"; $f.Size = New-Object Drawing.Size(520, 470)
    $f.StartPosition = "CenterScreen"; $f.FormBorderStyle = "FixedDialog"; $f.MaximizeBox = $false
    $f.Font = New-Object Drawing.Font("Segoe UI", 10)
    $y = 16
    function Label($t, $yy, $h = 22) { $l = New-Object Windows.Forms.Label; $l.Text = $t; $l.Location = New-Object Drawing.Point(20, $yy); $l.Size = New-Object Drawing.Size(470, $h); $f.Controls.Add($l); $l }
    $intro = Label "Backeddy Server dentro deste Windows: backup do Time Machine dos Macs e dos PCs da casa. Nada aqui é apagado." $y 44
    $y += 52
    Label "Nome do servidor (aparece no Mac e no Windows):" $y | Out-Null; $y += 24
    $tName = New-Object Windows.Forms.TextBox; $tName.Text = "Backup da Casa"; $tName.Location = New-Object Drawing.Point(20, $y); $tName.Width = 460; $f.Controls.Add($tName); $y += 36
    Label "Senha (mínimo 6 caracteres), duas vezes:" $y | Out-Null; $y += 24
    $tP1 = New-Object Windows.Forms.TextBox; $tP1.UseSystemPasswordChar = $true; $tP1.Location = New-Object Drawing.Point(20, $y); $tP1.Width = 225; $f.Controls.Add($tP1)
    $tP2 = New-Object Windows.Forms.TextBox; $tP2.UseSystemPasswordChar = $true; $tP2.Location = New-Object Drawing.Point(255, $y); $tP2.Width = 225; $f.Controls.Add($tP2); $y += 36
    Label "Unidade onde ficam os backups:" $y | Out-Null; $y += 24
    $cDrive = New-Object Windows.Forms.ComboBox; $cDrive.DropDownStyle = "DropDownList"; $cDrive.Location = New-Object Drawing.Point(20, $y); $cDrive.Width = 460; $f.Controls.Add($cDrive); $y += 36
    $drives = Get-PSDrive -PSProvider FileSystem | Where-Object { $_.Free -gt 20GB } | Sort-Object Free -Descending
    foreach ($d in $drives) { [void]$cDrive.Items.Add(("{0}:  {1:N0} GB livres" -f $d.Name, ($d.Free / 1GB))) }
    if ($cDrive.Items.Count -gt 0) { $cDrive.SelectedIndex = 0 }
    $lSize = Label "Tamanho do disco de backup (GB):" $y; $y += 24
    $nSize = New-Object Windows.Forms.NumericUpDown; $nSize.Location = New-Object Drawing.Point(20, $y); $nSize.Width = 120
    $nSize.Minimum = 20; $nSize.Maximum = 64000; $f.Controls.Add($nSize)
    $hint = Label "" ($y + 2); $hint.Location = New-Object Drawing.Point(150, ($y + 2)); $hint.Size = New-Object Drawing.Size(330, 40)
    $upd = {
        if ($cDrive.SelectedIndex -ge 0) {
            $d = $drives[$cDrive.SelectedIndex]; $free = [math]::Floor($d.Free / 1GB)
            $rec = [math]::Max(20, [math]::Floor($free * 0.8))
            $nSize.Maximum = [math]::Max(20, $free - 5); $nSize.Value = [math]::Min($rec, $nSize.Maximum)
            $hint.Text = "Recomendado: $rec GB (80% do livre). O arquivo cresce aos poucos, conforme os backups."
        }
    }
    $cDrive.Add_SelectedIndexChanged($upd); & $upd
    $y += 50
    $cQuiet = New-Object Windows.Forms.CheckBox; $cQuiet.Text = "Sem alarme das 22h às 8h (o Windows bipa se algo der errado)"; $cQuiet.Checked = $true
    $cQuiet.Location = New-Object Drawing.Point(20, $y); $cQuiet.Width = 470; $f.Controls.Add($cQuiet); $y += 40
    $ok = New-Object Windows.Forms.Button; $ok.Text = "Instalar"; $ok.Location = New-Object Drawing.Point(300, $y); $ok.Size = New-Object Drawing.Size(90, 32)
    $cancel = New-Object Windows.Forms.Button; $cancel.Text = "Cancelar"; $cancel.Location = New-Object Drawing.Point(395, $y); $cancel.Size = New-Object Drawing.Size(90, 32)
    $cancel.DialogResult = "Cancel"; $f.CancelButton = $cancel; $f.AcceptButton = $ok
    $f.Controls.Add($ok); $f.Controls.Add($cancel)
    $ok.Add_Click({
        if ($tName.Text.Trim() -eq "") { Msg "Escolha um nome." | Out-Null; return }
        if ($tP1.Text.Length -lt 6) { Msg "A senha precisa ter pelo menos 6 caracteres." | Out-Null; return }
        if ($tP1.Text -ne $tP2.Text) { Msg "As senhas não conferem." | Out-Null; return }
        if ($cDrive.SelectedIndex -lt 0) { Msg "Nenhuma unidade com pelo menos 20 GB livres." | Out-Null; return }
        $f.DialogResult = "OK"; $f.Close()
    })
    if ($f.ShowDialog() -ne "OK") { return $null }
    return [pscustomobject]@{
        Name = $tName.Text.Trim(); Password = $tP1.Text; Drive = $drives[$cDrive.SelectedIndex].Name
        SizeGB = [int]$nSize.Value; Quiet = $cQuiet.Checked
    }
}
$cfg = Show-Form
if ($null -eq $cfg) { exit }
$Dir = "$($cfg.Drive):\Backeddy"
New-Item -ItemType Directory -Force -Path $Dir | Out-Null
Log "nome=$($cfg.Name) pasta=$Dir tamanho=$($cfg.SizeGB)GB silêncio=$($cfg.Quiet)"

# ---------- 3. Imagem do sistema ----------
$Ext = if ($Motor -eq "hyperv") { "vhdx" } else { "vdi" }
$Sys = Join-Path $Dir "sistema.$Ext"
if (-not $Imagem) {
    try {
        $rel = Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/BackEddy/server/releases/latest"
        $asset = $rel.assets | Where-Object { $_.name -like "*-vm.$Ext" } | Select-Object -First 1
        if ($null -eq $asset) { throw "a versão $($rel.tag_name) não tem imagem de VM" }
        $dl = Join-Path $Dir $asset.name
        Log "baixando $($asset.browser_download_url)"
        $ProgressPreference = "SilentlyContinue"
        Invoke-WebRequest -UseBasicParsing $asset.browser_download_url -OutFile $dl
        $Imagem = $dl
    } catch {
        Log "download: $_"
        $ofd = New-Object Windows.Forms.OpenFileDialog
        $ofd.Title = "Não deu para baixar. Escolha o arquivo backeddy-server-...-vm.$Ext"
        $ofd.Filter = "Imagem da VM (*-vm.$Ext)|*-vm.$Ext|Todos (*.*)|*.*"
        if ($ofd.ShowDialog() -ne "OK") { exit }
        $Imagem = $ofd.FileName
    }
}
if (-not (Test-Path $Imagem)) { Falha "Imagem não encontrada: $Imagem" }
Copy-Item -Force $Imagem $Sys
Log "sistema: $Sys (de $Imagem)"

# ---------- 4. Disco de backup e disquinho de configuração ----------
$Data = Join-Path $Dir "backups.$Ext"
if (-not (Test-Path $Data)) {
    if ($Motor -eq "hyperv") { New-VHD -Path $Data -SizeBytes ([int64]$cfg.SizeGB * 1GB) -Dynamic | Out-Null }
    else { VBM createmedium disk --filename $Data --size ([int64]$cfg.SizeGB * 1024) --format VDI --variant Standard | Out-Null }
}

function New-ConfigDisk([string]$path, [string]$content) {
    if (Test-Path $path) { Remove-Item -Force $path }
    $ds = Join-Path $env:TEMP "backeddy-diskpart.txt"
    @("create vdisk file=""$path"" maximum=64 type=expandable", "select vdisk file=""$path""", "attach vdisk",
      "create partition primary", "format fs=fat32 label=BACKEDDYCFG quick", "assign") | Set-Content -Encoding ASCII $ds
    diskpart /s $ds | Out-Null
    Start-Sleep -Seconds 2
    $vol = Get-Volume -FileSystemLabel BACKEDDYCFG -ErrorAction SilentlyContinue | Where-Object DriveLetter | Select-Object -First 1
    if ($null -eq $vol) { throw "não achei o disquinho de configuração montado" }
    [IO.File]::WriteAllText("$($vol.DriveLetter):\firstboot.env", $content, (New-Object Text.UTF8Encoding($false)))
    @("select vdisk file=""$path""", "detach vdisk") | Set-Content -Encoding ASCII $ds
    diskpart /s $ds | Out-Null
    Remove-Item -Force $ds
}
$b64 = { param($s) [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($s)) }
$envText = "NAME_B64=$(& $b64 $cfg.Name)`nPASSWORD_B64=$(& $b64 $cfg.Password)`nTM_PERCENT=60`nDISKS=""auto""`nBEEP_OK=0`n"
if ($cfg.Quiet) { $envText += "QUIET_START=22`nQUIET_END=8`n" } else { $envText += "QUIET_START=`nQUIET_END=`n" }
$CfgDisk = Join-Path $Dir "configuracao.vhd"
try { New-ConfigDisk $CfgDisk $envText } catch { Falha "Não deu para criar o disco de configuração: $_" }
Log "configuração: $CfgDisk"

if ($Motor -eq "hyperv") {
# ---------- 5. Rede em ponte (o Mac precisa ver o servidor na rede) ----------
$sw = Get-VMSwitch -SwitchType External -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $sw) {
    $route = Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1
    $nic = Get-NetAdapter -InterfaceIndex $route.ifIndex
    Msg ("Vou criar uma rede virtual em ponte na placa ""$($nic.Name)"".`nA internet deste PC pode cair por alguns segundos.") | Out-Null
    $sw = New-VMSwitch -Name "Backeddy (ponte)" -NetAdapterName $nic.Name -AllowManagementOS $true
    Log "switch criado: $($sw.Name) em $($nic.Name)"
}

# ---------- 6. Máquina virtual ----------
New-VM -Name $VmName -Generation 2 -MemoryStartupBytes 512MB -SwitchName $sw.Name -Path $Dir -NoVHD | Out-Null
Set-VMMemory $VmName -DynamicMemoryEnabled $false
Set-VMProcessor $VmName -Count 2
Set-VMFirmware $VmName -SecureBootTemplate MicrosoftUEFICertificateAuthority
Set-VM $VmName -AutomaticCheckpointsEnabled $false -AutomaticStartAction Start -AutomaticStopAction ShutDown -AutomaticStartDelay 30
Add-VMHardDiskDrive $VmName -Path $Sys
Add-VMHardDiskDrive $VmName -Path $Data
Add-VMHardDiskDrive $VmName -Path $CfgDisk
Set-VMFirmware $VmName -FirstBootDevice (Get-VMHardDiskDrive $VmName | Where-Object Path -eq $Sys)
Start-VM $VmName
Log "VM criada e ligada"

}
else {
# ---------- 5/6. VirtualBox: rede em ponte, VM e início automático ----------
$route = Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1
$nic = Get-NetAdapter -InterfaceIndex $route.ifIndex
$bridge = $nic.InterfaceDescription
$names = (& $VBox list bridgedifs) | Where-Object { $_ -match '^Name:\s+(.+)$' } | ForEach-Object { $Matches[1].Trim() }
if ($names -notcontains $bridge -and $names.Count -gt 0) { $bridge = $names | Where-Object { $_ -like "*$($nic.Name)*" } | Select-Object -First 1; if (-not $bridge) { $bridge = $names[0] } }
VBM createvm --name $VmName --ostype Debian_64 --register --basefolder $Dir | Out-Null
VBM modifyvm $VmName --memory 512 --cpus 2 --firmware efi --nic1 bridged --bridgeadapter1 $bridge `
    --graphicscontroller vmsvga --audio-enabled off --boot1 disk --boot2 none --boot3 none --boot4 none | Out-Null
VBM storagectl $VmName --name SATA --add sata --controller IntelAhci --portcount 3 | Out-Null
VBM storageattach $VmName --storagectl SATA --port 0 --device 0 --type hdd --medium $Sys | Out-Null
VBM storageattach $VmName --storagectl SATA --port 1 --device 0 --type hdd --medium $Data | Out-Null
VBM storageattach $VmName --storagectl SATA --port 2 --device 0 --type hdd --medium $CfgDisk | Out-Null
VBM startvm $VmName --type headless | Out-Null
Log "VM do VirtualBox criada e ligada (ponte: $bridge)"
# liga junto com o Windows (e ao entrar no Windows, se não tiver ligado antes)
$act = New-ScheduledTaskAction -Execute $VBox -Argument "startvm `"$VmName`" --type headless"
$trg = @((New-ScheduledTaskTrigger -AtStartup), (New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME))
$pri = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType S4U -RunLevel Highest
Register-ScheduledTask -TaskName "Backeddy Server (VirtualBox)" -Action $act -Trigger $trg -Principal $pri -Force | Out-Null
$vbMac = ((& $VBox showvminfo $VmName --machinereadable) | Where-Object { $_ -match '^macaddress1="(.+)"' } | ForEach-Object { $Matches[1] }) -replace '(..)(?!$)', '$1-'

}

# ---------- 7. Esperar o servidor ficar pronto ----------
$slug = ($cfg.Name.Normalize([Text.NormalizationForm]::FormD) -replace '\p{Mn}', '').ToLower() -replace '[^a-z0-9]+', '-'
$slug = $slug.Trim('-'); if (-not $slug) { $slug = "backeddy" }
$wait = New-Object Windows.Forms.Form
$wait.Text = "Backeddy Server"; $wait.Size = New-Object Drawing.Size(460, 160); $wait.StartPosition = "CenterScreen"
$wait.FormBorderStyle = "FixedDialog"; $wait.ControlBox = $false; $wait.Font = New-Object Drawing.Font("Segoe UI", 10)
$wl = New-Object Windows.Forms.Label; $wl.Location = New-Object Drawing.Point(20, 20); $wl.Size = New-Object Drawing.Size(410, 80)
$wl.Text = "Preparando o servidor (uns 5 minutos)…"; $wait.Controls.Add($wl); $wait.Show(); [Windows.Forms.Application]::DoEvents()
$ip = $null; $ready = $false; $t0 = Get-Date
while (((Get-Date) - $t0).TotalMinutes -lt 20) {
    if ($Motor -eq "hyperv") {
        $ip = (Get-VMNetworkAdapter -VMName $VmName).IPAddresses | Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } | Select-Object -First 1
    } else {
        # sem integração com o VirtualBox: acha o IP pela placa de rede (MAC) ou pelo nome na rede (.local)
        $ip = (Get-NetNeighbor -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.LinkLayerAddress -eq $vbMac.ToUpper() } | Select-Object -First 1).IPAddress
        if (-not $ip) { try { $ip = ([Net.Dns]::GetHostAddresses("$slug.local") | Where-Object AddressFamily -eq "InterNetwork" | Select-Object -First 1).IPAddressToString } catch {} }
    }
    $min = [math]::Floor(((Get-Date) - $t0).TotalMinutes)
    if ($ip) {
        $wl.Text = "Preparando o servidor (uns 5 minutos)…`nEndereço: $ip · $min min"
        try { $r = Invoke-WebRequest -UseBasicParsing -TimeoutSec 3 "http://$ip/"; if ($r.StatusCode -eq 200) { $ready = $true; break } } catch {}
    } else { $wl.Text = "Ligando a máquina virtual… · $min min" }
    [Windows.Forms.Application]::DoEvents(); Start-Sleep -Seconds 5
}
$wait.Close()
Log "pronto=$ready ip=$ip"

# ---------- 8. Vigia no Windows: bipa e avisa quando a VM tiver alerta (a VM não tem alto-falante) ----------
try {
    $vsrc = Join-Path $PSScriptRoot "Backeddy-Vigia.ps1"
    if (-not (Test-Path $vsrc)) { $vsrc = Join-Path $Base "Backeddy-Vigia.ps1" }
    $vdst = Join-Path $Base "Backeddy-Vigia.ps1"
    if ($vsrc -ne $vdst) { Copy-Item -Force $vsrc $vdst }
    @{ host = "$slug.local"; ip = "$ip" } | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Base "vigia.json")
    $va = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$vdst`""
    $vt = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
    $vp = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
    $vs = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    Register-ScheduledTask -TaskName "Backeddy Vigia" -Action $va -Trigger $vt -Principal $vp -Settings $vs -Force | Out-Null
    Start-ScheduledTask -TaskName "Backeddy Vigia"
    Log "vigia instalado"
} catch { Log "vigia: $_" }
if ($ready) {
    Msg ("Pronto! ""$($cfg.Name)"" está funcionando.`n`n" +
        "Mac: Ajustes do Sistema › Geral › Time Machine › ""$($cfg.Name)"" (usuário: backeddy)`n" +
        "Painel: http://$slug.local (ou http://$ip)`n" +
        "Acesso: ssh admin@$slug.local`n`nA máquina virtual liga sozinha junto com o Windows.`n" +
        "Se algo der errado, este Windows bipa e mostra um aviso (ícone do escudo, perto do relógio).") | Out-Null
    Start-Process "http://$ip/"
} else {
    Msg ("A máquina virtual foi criada, mas o servidor ainda não respondeu.`n" +
        "Abra o Gerenciador do Hyper-V ou o VirtualBox, conecte em ""$VmName"" e veja a tela.`n`nDetalhes em: $Log") "Backeddy Server" "Warning" | Out-Null
}
