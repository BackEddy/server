# Testando a imagem do pendrive (0.4)

A imagem é montada no GitHub Actions e testada no Hyper-V, sem pendrive: o arquivo `.vhdx`
faz o papel do pendrive. Nesta versão: **só UEFI** (PCs de ~2012 para cá) e **só rede por cabo**.

## 1. Montar a imagem no GitHub

1. github.com/BackEddy/server › aba **Actions** › **Imagem do pendrive** › **Run workflow**
   (ou `gh workflow run image.yml -R BackEddy/server`). Leva uns 5 minutos.
2. Se falhar, abra o passo vermelho, copie o final do log e mande para o Claude.
3. Em **Artifacts**, baixe `backeddy-server-imagem` (logado no GitHub; o botão de baixar fica na ponta
   direita da linha) ou: `gh run download ID -R BackEddy/server -D B:\Backeddy`.
   Descompacte o `.vhdx` em `B:\Backeddy\`.

Publicar uma tag (`git tag v0.4.0 && git push --tags`) faz o mesmo e anexa os arquivos numa release.

## 2. Criar uma VM de teste "PC largado"

PowerShell como Administrador. Se já existir a VM de teste anterior, apague antes:
`Remove-VM Backeddy-Teste-Pendrive -Force` e a pasta `B:\Backeddy\teste-pendrive`.

```powershell
$vm     = "Backeddy-Teste-Pendrive"
$pasta  = "B:\Backeddy\teste-pendrive"
$vhdx   = "B:\Backeddy\backeddy-server-0.4.0.vhdx"
New-Item -ItemType Directory -Force -Path $pasta | Out-Null
New-VM -Name $vm -Generation 2 -MemoryStartupBytes 1GB -SwitchName Bridge_Rede -Path $pasta -NoVHD
Set-VMProcessor $vm -Count 2
Set-VMMemory $vm -DynamicMemoryEnabled $false
Set-VMFirmware $vm -SecureBootTemplate MicrosoftUEFICertificateAuthority
Set-VM $vm -AutomaticCheckpointsEnabled $false
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

Sem memória livre para 1 GB: use `-MemoryStartupBytes 512MB`.

## 3. O que conferir no instalador (gráfico)

- [ ] Abre uma janela clara no estilo Mac, com as etapas na lateral (Introdução › … › Resumo)
- [ ] Setas ← → trocam o botão azul; Enter confirma; botão principal à direita
- [ ] "Encontrei estes discos": SSD 24 GB para o sistema, 2 discos espelhados, **sem o pendrive**
- [ ] Aviso vermelho com "Desligar" selecionado; depois digitar **APAGAR**, nome e senha duas vezes
- [ ] Barra de progresso com o spinner girando (não pode parecer travado)
- [ ] "Pronto!" com o texto "Retire o pendrive e aperte Enter para reiniciar"

Na tela "Pronto!", **teste o reinício automático**: com a VM ligada, tire o "pendrive":
```powershell
Get-VMHardDiskDrive $vm | Where-Object Path -eq $vhdx | Remove-VMHardDiskDrive
```
- [ ] Aparece "Reiniciando…" e a VM reinicia sozinha, já pelo disco de 24 GB

Se ela tentar iniciar pela rede ou parar numa tela do firmware, desligue e acerte a ordem:
```powershell
Stop-VM $vm -TurnOff
Set-VMFirmware $vm -FirstBootDevice (Get-VMHardDiskDrive $vm | Select-Object -First 1)
Start-VM $vm
```

## 4. Primeiro boot e boots seguintes

- [ ] Fonte pequena (padrão) e **sem** as mensagens "systemd-ssh-generator … AF_VSOCK"
- [ ] Tela limpa "Preparando os discos de backup e os serviços…" com tempo correndo
- [ ] No fim, a tela mostra **"Backup da Casa — servidor funcionando"** (verde), como conectar Mac e Windows,
      `ssh admin@backup-da-casa.local (ou admin@IP)` e o diagnóstico com `[  OK  ]` verdes
- [ ] Reinicie a VM (`Restart-VM $vm -Force`): a mesma tela volta, atualizada
- [ ] No Mac, "Backup da Casa" aparece no Time Machine (desligue antes outras VMs com o mesmo nome)
- [ ] `ssh admin@backup-da-casa.local` e `sudo backeddy status`: no Mac aparece com emojis 🟢

## Se algo der errado

- Instalador gráfico não abriu (tela preta ou modo texto): tire uma foto. Alt+F2 abre outro console
  (usuário `root`, sem senha): `cat /tmp/backeddy-install.log` e `ls /dev/fb*`.
  Para usar o modo texto: `backeddy-installer --text`.
- Primeiro boot: entre como `admin` e rode `sudo cat /var/log/backeddy-firstboot.log`.
