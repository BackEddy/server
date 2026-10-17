# Testando os discos do Backeddy Server (espelho, defeito e troca)

Usa a mesma VM do [TESTING.pt-BR.md](TESTING.pt-BR.md), com discos virtuais pequenos
(nada de backup grande). Ordem dos testes:

| # | Cenário | O que deve acontecer |
| --- | --- | --- |
| 0 | Atualizar para a 0.2 e reiniciar | Tudo volta sozinho depois do `reboot` |
| 1 | Dois discos de tamanhos diferentes | Espelho criado, com aviso de tamanhos diferentes |
| 2 | Um disco "morre" | Servidor continua funcionando; status mostra espelho DEGRADADO |
| 3 | Troca do disco | `backeddy replace-disk` reconstrói o espelho; status volta ao verde |

Sempre confira os nomes com `lsblk` antes de rodar comandos que apagam discos:
eles podem mudar quando discos entram ou saem.

## 0. Atualizar a VM para a 0.2 e testar a reinicialização

No Mac:
```bash
scp -rp ~/backeddy-server eddy@192.168.0.67:
```
Na VM, como root:
```bash
cd /home/eddy/backeddy-server
bash install.sh --name "Backup da Casa" --disk /dev/sdb   # mantém o disco (não apaga)
reboot
```
Depois de reiniciar (espere ~1 minuto, entre de novo por SSH e `su -`):
```bash
backeddy status
```
- [ ] Tudo 🟢, incluindo a linha "1 disco(s), sem espelho"

## 1. Espelho com dois discos de tamanhos diferentes

No Windows, PowerShell como Administrador:
```powershell
$vm = "Backeddy-Server"
Stop-VM $vm
Get-VMHardDiskDrive $vm | Where-Object Path -like '*backups.vhdx' | Remove-VMHardDiskDrive
New-VHD -Path "B:\Backeddy\disco-a.vhdx" -SizeBytes 20GB -Dynamic | Out-Null
New-VHD -Path "B:\Backeddy\disco-b.vhdx" -SizeBytes 30GB -Dynamic | Out-Null
Add-VMHardDiskDrive $vm -Path "B:\Backeddy\disco-a.vhdx"
Add-VMHardDiskDrive $vm -Path "B:\Backeddy\disco-b.vhdx"
Start-VM $vm
```
(O `backups.vhdx` de 1 TB só sai da VM; o arquivo continua no B:.)

Na VM, como root:
```bash
lsblk                                   # deve mostrar sdb 20G e sdc 30G
backeddy status                         # esperado: 🔴 discos NÃO montados (os antigos saíram)
cd /home/eddy/backeddy-server
bash install.sh --name "Backup da Casa" --disk /dev/sdb --disk /dev/sdc
backeddy status
```
- [ ] O instalador mostra "2 discos ESPELHADOS · ~20 GB protegidos" e o aviso de tamanhos diferentes
- [ ] O status mostra 🟢 "Espelho de 2 discos"
- [ ] (Opcional) O Mac ainda enxerga "Backup da Casa" na lista do Time Machine

## 2. Um disco "morre"

No PowerShell:
```powershell
Stop-VM $vm
Get-VMHardDiskDrive $vm | Where-Object Path -like '*disco-b.vhdx' | Remove-VMHardDiskDrive
Start-VM $vm
```
A VM demora um pouco mais para ligar (espera ~30 s pelo disco que sumiu). Na VM:
```bash
backeddy status
```
- [ ] Os serviços ficam 🟢 (o servidor continua funcionando)
- [ ] Aparece 🔴 "Espelho DEGRADADO: falta um disco"
- [ ] `journalctl -t backeddy` mostra o aviso de modo degradado

## 3. Troca do disco

O disco novo pode ser colocado com a VM ligada. Ele precisa ser **do mesmo tamanho ou maior**
que o que morreu (30 GB). No PowerShell:
```powershell
New-VHD -Path "B:\Backeddy\disco-c.vhdx" -SizeBytes 30GB -Dynamic | Out-Null
Add-VMHardDiskDrive $vm -Path "B:\Backeddy\disco-c.vhdx"
```
Na VM:
```bash
lsblk                                   # descubra o nome do disco novo de 30G sem partições
backeddy replace-disk /dev/sdX          # troque sdX pelo nome certo; pede APAGAR
backeddy status
```
- [ ] A reconstrução termina com "✅ Espelho reconstruído"
- [ ] O status volta a 🟢 "Espelho de 2 discos"
- [ ] Depois de um `reboot`, continua tudo 🟢

## Limpeza

Depois dos testes, para voltar ao disco de 1 TB:
```powershell
Stop-VM $vm
Get-VMHardDiskDrive $vm | Where-Object Path -like '*disco-*.vhdx' | Remove-VMHardDiskDrive
Add-VMHardDiskDrive $vm -Path "B:\Backeddy\backups.vhdx"
Start-VM $vm
Remove-Item "B:\Backeddy\disco-*.vhdx"
```
E na VM: `bash install.sh --name "Backup da Casa" --disk /dev/sdb` (como o servidor agora
procura o espelho, ele vai propor formatar o disco de 1 TB de novo: pode confirmar, ele está vazio).

## Se algo der errado

```bash
backeddy status
journalctl -t backeddy -b --no-pager
btrfs filesystem show /srv/backeddy
journalctl -u backeddy-mount -u smbd -b -n 60 --no-pager
```
