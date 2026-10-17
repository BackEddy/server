# Testando a imagem do pendrive (0.3)

A imagem é montada no GitHub Actions e testada no Hyper-V, sem pendrive: o arquivo `.vhdx`
faz o papel do pendrive. Nesta versão: **só UEFI** (PCs de ~2012 para cá) e **só rede por cabo**.

## 1. Montar a imagem no GitHub

1. github.com/BackEddy/server › aba **Actions** › **Imagem do pendrive** › **Run workflow**.
2. Leva uns 20–30 minutos. Se falhar, abra o passo vermelho, copie o final do log e mande para o Claude.
3. No fim da execução, em **Artifacts**, baixe `backeddy-server-imagem` (um .zip com o `.vhdx`,
   o `.img.xz` e o `SHA256SUMS`). Descompacte o `.vhdx` no PC do Hyper-V (ex.: `B:\Backeddy\`).

Publicar uma tag (`git tag v0.3.0 && git push --tags`) faz o mesmo e anexa os arquivos numa release.

## 2. Criar uma VM de teste "PC largado"

PowerShell como Administrador (ajuste o caminho do `.vhdx` baixado):

```powershell
$vm     = "Backeddy-Teste-Pendrive"
$pasta  = "B:\Backeddy\teste-pendrive"
$vhdx   = "B:\Backeddy\backeddy-server-0.3.0.vhdx"     # o "pendrive"
$switch = "Bridge_Rede"

New-Item -ItemType Directory -Force -Path $pasta | Out-Null
New-VM -Name $vm -Generation 2 -MemoryStartupBytes 1GB -SwitchName $switch -Path $pasta -NoVHD
Set-VMProcessor $vm -Count 2
Set-VMMemory $vm -DynamicMemoryEnabled $false
Set-VMFirmware $vm -SecureBootTemplate MicrosoftUEFICertificateAuthority
Set-VM $vm -AutomaticCheckpointsEnabled $false

# "pendrive" primeiro, depois os discos do PC: um SSD pequeno e dois HDs iguais (receita com espelho)
Add-VMHardDiskDrive $vm -Path $vhdx
New-VHD -Path "$pasta\ssd.vhdx" -SizeBytes 24GB -Dynamic | Out-Null
New-VHD -Path "$pasta\hd1.vhdx" -SizeBytes 60GB -Dynamic | Out-Null
New-VHD -Path "$pasta\hd2.vhdx" -SizeBytes 60GB -Dynamic | Out-Null
Add-VMHardDiskDrive $vm -Path "$pasta\ssd.vhdx"
Add-VMHardDiskDrive $vm -Path "$pasta\hd1.vhdx"
Add-VMHardDiskDrive $vm -Path "$pasta\hd2.vhdx"
Set-VMFirmware $vm -FirstBootDevice (Get-VMHardDiskDrive $vm | Where-Object Path -eq $vhdx)

Start-VM $vm
vmconnect localhost $vm
```

Sem memória livre para 1 GB: use `-MemoryStartupBytes 512MB` (o instalador é leve).
No Hyper-V todos os discos parecem "HD", então a receita escolhida será "discos parecidos"
(sistema numa parte do primeiro + espelho) — tudo bem para o teste.

## 3. O que conferir no instalador

- [ ] Liga direto no instalador, com letras grandes (sem login)
- [ ] "Encontrei estes discos": mostra a receita e **não lista o próprio pendrive**
- [ ] Aviso vermelho com a lista de discos, "Desligar" já selecionado
- [ ] Pede **APAGAR**, depois nome (já preenchido "Backup da Casa") e senha duas vezes
- [ ] Barra de etapas até "Pronto!"

Ao chegar em "Pronto!", **antes** de apertar Enter, tire o "pendrive":
```powershell
Stop-VM $vm -TurnOff
Get-VMHardDiskDrive $vm | Where-Object Path -eq $vhdx | Remove-VMHardDiskDrive
Set-VMFirmware $vm -FirstBootDevice (Get-VMHardDiskDrive $vm | Select-Object -First 1)
Start-VM $vm
```

## 4. Primeiro boot

- [ ] A tela mostra "Preparando os discos de backup..." e, depois de alguns minutos, "Pronto!" com o endereço
- [ ] No Mac, "Backup da Casa" aparece na lista do Time Machine
      (desligue antes a VM Backeddy-Server de teste, para não haver dois servidores com o mesmo nome)
- [ ] `ssh admin@backup-da-casa.local` (senha escolhida) e `sudo backeddy status` com tudo 🟢 e "Espelho de 2 discos"

## Se algo der errado

- No instalador: tire uma foto da tela do erro. O log fica em `/tmp/backeddy-install.log`:
  Alt+F2 abre outro console (usuário `root`, sem senha) e `tail -50 /tmp/backeddy-install.log`.
- No primeiro boot: entre como `admin` e rode `sudo cat /var/log/backeddy-firstboot.log`.
