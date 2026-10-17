# Testando o Backeddy Server no Hyper-V

Objetivo: criar um servidor Time Machine novo, **ao lado** do atual, e comparar.
Nada aqui mexe na VM antiga nem no disco dela.

> 💡 O Time Machine aceita **vários destinos**. Você pode adicionar o servidor novo sem remover o antigo:
> o Mac alterna entre os dois. Só desligue o antigo quando o novo estiver aprovado.

## 1. Baixar o Debian

Baixe a ISO **netinst amd64** do Debian 13 em <https://www.debian.org/download> (~700 MB).

## 2. Criar a VM (Gerenciador do Hyper-V › Novo › Máquina Virtual)

| Etapa | Valor |
| --- | --- |
| Geração | **Geração 2** |
| Memória | 2048 MB |
| Rede | O **mesmo switch externo** da VM do Time Machine atual |
| Disco | Novo VHDX de **16 GB** (só para o sistema) |
| Instalação | A ISO do Debian baixada |

**Antes de ligar**, nas Configurações da VM:

1. **Segurança** › Modelo de Inicialização Segura: **Autoridade de Certificação UEFI da Microsoft** (o padrão "Windows" não liga o Debian).
2. **Controlador SCSI** › Disco Rígido › **Novo**: o disco dos backups.
   - Mais simples: VHDX **de tamanho fixo** (é mais rápido que o dinâmico), por exemplo 1 TB.
   - Mais rápido: um **disco físico inteiro** (antes, deixe-o *Offline* no Gerenciamento de Disco do Windows). **Nunca** use o disco da VM antiga.
3. Processador: 2 núcleos.

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

# jeito B: copiar do Mac (use -rp para manter as permissões)
scp -rp ~/backeddy-server seu-usuario@IP:
ssh seu-usuario@IP
su -
cd /home/seu-usuario/backeddy-server
```

Em seguida, ainda como root:

```bash
lsblk        # confira: o de 16 GB é o sistema (sda); o grande é o dos backups (provavelmente sdb)
bash install.sh --name "Backup da Casa" --disk /dev/sdb
```

> Com pouca memória livre no Windows, a VM pode não ligar com 2 GB. Use 512 MB
> (`Set-VMMemory Backeddy-Server -StartupBytes 512MB`): o instalador do Debian entra em
> "low memory mode" (em inglês) e funciona. No menu de componentes, marque `choose-mirror`,
> `nic-modules`, `partman-auto`, `partman-efi` e `partman-ext3`.

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
