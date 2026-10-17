# Testando o Backeddy em VM no Windows (0.6.1)

Para quem já tem um PC/servidor com Windows ligado o tempo todo. O instalador cria uma máquina
virtual que dá partida pelo "pendrive virtual" (a mesma imagem do pendrive de verdade). A instalação
é a mesma do pendrive, na tela da VM. Nada no Windows é formatado.

## 1. Preparar

1. Rode o workflow **Imagem do pendrive** e baixe o artefato `backeddy-server-imagem`.
   Vêm o `backeddy-server-0.6.1.vhdx` (Hyper-V) e o `backeddy-server-0.6.1.vdi` (VirtualBox).
2. Baixe o repositório (github.com/BackEddy/server › Code › Download ZIP) e descompacte.

## 2. Instalar

Rode `windows\Instalar Backeddy.cmd -Imagem caminho\backeddy-server-0.6.1.vhdx` (ou só clique duas
vezes no .cmd e escolha o arquivo quando ele pedir).

- [ ] Checa a BIOS e o Hyper-V (liga com 1 reinício, ou usa o VirtualBox)
- [ ] Janela pequena: só a unidade e o tamanho do disco de backup (mínimo 40 GB)
- [ ] A VM liga e a tela dela abre sozinha no instalador gráfico (mouse funciona)
- [ ] No instalador: cartão **"Disco de backup da máquina virtual"** (verde, sem aviso de risco),
      APAGAR, nome, senha, **silêncio à noite** (sem teste de bipe: quem bipa é o Windows)
- [ ] Na tela "Pronto!", clicar em **Ejetar pendrive** na janelinha do Windows → a VM reinicia sozinha
- [ ] "Preparando o servidor…" mostra o endereço e, em ~5 minutos, "Pronto!" com o nome e abre o painel
- [ ] Ícone do escudo perto do relógio: "Backeddy: tudo certo"

## 3. Conferir

- [ ] No Mac, o servidor aparece no Time Machine; `ssh admin@nome.local` entra
- [ ] Reinicie o Windows: a VM volta sozinha e o escudo também
- [ ] Alerta de teste: por SSH, `sudo systemctl stop smbd`; em ~2 minutos o Windows bipa e avisa;
      no painel, "Amanhã" reconhece; depois `sudo systemctl start smbd`

## Desfazer

Hyper-V: `Stop-VM "Backeddy Server" -TurnOff; Remove-VM "Backeddy Server" -Force` e apague a pasta
`X:\Backeddy`. VirtualBox: apague a VM e a tarefa "Backeddy Server (VirtualBox)".
Vigia: apague a tarefa "Backeddy Vigia" e a pasta `C:\ProgramData\Backeddy`.
