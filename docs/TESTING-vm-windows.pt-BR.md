# Testando o Backeddy em VM no Windows (0.6)

Para quem já tem um PC/servidor com Windows ligado o tempo todo: o instalador cria uma máquina
virtual pronta. Nada é formatado.

## 1. Preparar

1. Rode o workflow **Imagem do pendrive** (ou publique uma tag `v0.6.0`).
2. Baixe o artefato `backeddy-server-imagem` e descompacte. Vão aparecer, entre outros:
   `backeddy-server-0.6.0-vm.vhdx` (Hyper-V) e `backeddy-server-0.6.0-vm.vdi` (VirtualBox).
3. Copie a pasta `windows/` do repositório para o PC com Windows (ou baixe o repositório em .zip).

## 2. Instalar

Clique duas vezes em **`Instalar Backeddy.cmd`** e aceite o pedido de administrador.

- [ ] Se a virtualização estiver desligada na BIOS, ele explica o que ligar e para
- [ ] Hyper-V desligado: pergunta se liga (reinicia uma vez e continua sozinho) ou usa o VirtualBox
- [ ] Janela única: nome, senha 2x, unidade com espaço livre, tamanho sugerido, silêncio à noite
- [ ] Sem release publicada, pede o arquivo `…-vm.vhdx` (ou `.vdi`): escolha o que você baixou
- [ ] Sem switch externo, avisa e cria uma ponte (a internet pisca)
- [ ] "Preparando o servidor…" mostra o IP e, em ~5 minutos, "Pronto!" e abre o painel

Atenção no PC do Eddy: já existe outra VM com o nome do servidor? Use um nome diferente no teste
(ex.: "Backup Teste") para não confundir com a "TimeMachine" de verdade.

## 3. Conferir

- [ ] Painel abre em `http://nome.local` com a senha; "Tudo certo"
- [ ] No Mac, o servidor aparece no Time Machine
- [ ] Ícone do escudo perto do relógio do Windows: "Backeddy: tudo certo"
- [ ] Reinicie o Windows: a VM volta sozinha e o escudo também
- [ ] Alerta de teste: entre por SSH e rode `sudo systemctl stop smbd`; em até ~2 minutos o Windows
      bipa e mostra o aviso; no painel, "Amanhã" reconhece; depois `sudo systemctl start smbd`

## Desfazer

Hyper-V: `Stop-VM "Backeddy Server" -TurnOff; Remove-VM "Backeddy Server" -Force` e apague a pasta
`X:\Backeddy`. VirtualBox: apague a VM no VirtualBox e a tarefa "Backeddy Server (VirtualBox)".
Vigia: apague a tarefa "Backeddy Vigia" e a pasta `C:\ProgramData\Backeddy`.
