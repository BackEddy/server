# Testando o Backeddy Server no Hyper-V

Objetivo: criar um servidor Time Machine novo, **ao lado** do atual, e comparar.
Nada aqui mexe na VM antiga nem no disco dela.

> 💡 O Time Machine aceita **vários destinos**. Você pode adicionar o servidor novo sem remover o antigo:
> o Mac alterna entre os dois. Só desligue o antigo quando o novo estiver aprovado.

## 1. Baixar o Debian

Baixe a ISO **netinst amd64** do Debian 13 em <https://www.debian.org/download> (~700 MB).

## 2. Criar a VM

PowerShell **como Administrador** (ajuste as duas primeiras linhas):

```powershell
$pasta = "B:\Backeddy"                                               # disco com espaço livre
$iso   = "C:\Users\SEU-USUARIO\Downloads\debian-13.4.0-amd64-netinst.iso"
$vm    = "Backeddy-Server"

New-Item -ItemType Directory -Force -Path $pasta | Out-Null
New-VM -Name $vm -Generation 2 -MemoryStartupBytes 1GB -SwitchName "NOME-DO-SWITCH-EXTERNO" `
  -Path $pasta -NewVHDPath "$pasta\sistema.vhdx" -NewVHDSizeBytes 16GB
Set-VMProcessor $vm -Count 2
Set-VMMemory $vm -DynamicMemoryEnabled $false
Set-VMFirmware $vm -SecureBootTemplate MicrosoftUEFICertificateAuthority
Set-VM $vm -AutomaticCheckpointsEnabled $false -AutomaticStartAction Start -AutomaticStopAction ShutDown
New-VHD -Path "$pasta\backups.vhdx" -SizeBytes 1TB -Dynamic | Out-Null
Add-VMHardDiskDrive $vm -Path "$pasta\backups.vhdx"
Add-VMDvdDrive $vm -Path $iso
Set-VMFirmware $vm -FirstBootDevice (Get-VMDvdDrive $vm)
Start-VM $vm
vmconnect localhost $vm
```

- **Checkpoints automáticos desligados** (`-AutomaticCheckpointsEnabled $false`): o Windows 11 cria um
  a cada vez que a VM liga, e aí os discos viram arquivos `.avhdx` de diferenças que crescem sem parar —
  péssimo para um servidor de backup. Se já existirem: `Get-VMCheckpoint $vm | Remove-VMCheckpoint`.
- **Memória**: se der "memória insuficiente", use `Set-VMMemory $vm -StartupBytes 512MB`. O instalador do
  Debian entra em "low memory mode" (em inglês) e funciona; o servidor roda bem com 512 MB.
- Nome do switch externo: Gerenciador do Hyper-V › Gerenciador de Comutador Virtual (o mesmo da VM antiga).
- Disco fixo (`-Fixed`) é mais rápido que o dinâmico; o dinâmico serve para testes.

## 3. Instalar o Debian

- Idioma: Português (Brasil). Nome da máquina: qualquer um (o script troca depois).
- Defina uma senha de **root** e crie seu usuário.
- Particionamento: **guiado, disco inteiro** — escolha o disco de **16 GB** (não o grande!).
- Seleção de software: **desmarque** "Ambiente de área de trabalho" e "GNOME"; **marque** "Servidor SSH" e "Utilitários padrão do sistema".

## 4. Rodar o Backeddy Server

Descubra o IP da VM no console dela com `hostname -I`. Depois, do Terminal do Mac:

```bash
# jeito A: baixar do GitHub (dentro da VM)
ssh seu-usuario@IP
su -
wget -qO- https://github.com/BackEddy/server/archive/refs/heads/main.tar.gz | tar xz
cd server-main

# jeito B: copiar do Mac (sobrescreve no lugar certo, também para atualizar depois)
tar -C ~ -czf - backeddy-server | ssh seu-usuario@IP tar -xzf -
ssh seu-usuario@IP
su -
cd /home/seu-usuario/backeddy-server
```

Em seguida, ainda como root:

```bash
lsblk        # confira: o de 16 GB é o sistema (sda); o grande é o dos backups (provavelmente sdb)
bash install.sh --name "Backup da Casa" --disk /dev/sdb
```

> Em "low memory mode", o instalador mostra um menu de componentes: marque `choose-mirror`,
> `nic-modules`, `partman-auto`, `partman-efi` e `partman-ext3`. Avisos do `tar` sobre
> "unknown extended header" (metadados do macOS) podem ser ignorados.

O script pede a senha do servidor (a mesma pode ser usada de novo se rodar outra vez) e, para apagar o disco, pede para digitar **APAGAR**.
No final mostra o resumo com os endereços.

## 5. Conectar o Mac

Ajustes do Sistema › Geral › Time Machine › **+** (Adicionar disco de backup):

- [ ] **"Backup da Casa" aparece sozinho na lista?** (é o teste mais importante)
- [ ] Usuário `backeddy` e a senha definida funcionam
- [ ] O primeiro backup começa e o Backeddy for Mac mostra o progresso

Se não aparecer na lista: Finder › Ir › Conectar ao Servidor › `smb://backup-da-casa.local` e tente de novo.

## 6. O que observar

- [ ] **Velocidade** do primeiro backup comparada ao servidor antigo (o Backeddy for Mac mostra a taxa de envio)
- [ ] Reinicie a VM no meio de um backup: o Mac retoma sozinho depois?
- [ ] Mac dormindo e acordando: os backups automáticos continuam?
- [ ] `sudo backeddy status` mostra tudo verde?
- [ ] (Opcional) PC Windows: `sudo backeddy add-pc NOME` e conectar com o KopiaUI

## Se algo der errado

Me mande a saída destes comandos:

```bash
sudo backeddy status
sudo journalctl -u smbd -u avahi-daemon -u backeddy-kopia -n 80 --no-pager
```
