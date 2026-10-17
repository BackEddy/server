# Contexto do projeto Backeddy

> Arquivo de memória do projeto: tudo o que foi decidido e feito nas conversas com o Claude.
> Leia antes de continuar o trabalho. Atualizado em 16/10/2026 (23h40).

## Como trabalhamos (combinado com o Eddy)

- **Sempre perguntar antes de fazer** (criar/alterar arquivos, código, docs, GitHub, máquina do Eddy).
  Planejar bem, depois executar num lote grande ("tacada").
- **Publicação no GitHub: opção B** — o Claude prepara tudo no Mac (`~/...`), o Eddy roda os comandos `git`/`gh`.
- Os ambientes do Claude **não acessam os espelhos do Debian nem têm KVM**: imagens Linux são montadas
  no GitHub Actions; testes no Hyper-V do Eddy.
- Lembrar o Eddy, de tempos em tempos, de atualizar o `contexto.md` no Claude Code.
- Especificação viva (Claude Docs): "Backeddy — Especificação"
  https://claude.ai/code/artifact/4a8a0051-86cc-4939-9fef-42aae446b824

## A família Backeddy

Nome: **Backeddy** = "back eddy", o redemoinho em que a água corre para trás (back + voltar no tempo + Eddy).
Domínio: **backeddy.com.br** (comprado; redirecionar para o GitHub quando o DNS propagar).
Organização GitHub: **BackEddy** (github.com/BackEddy), página de apresentação no repo `BackEddy/.github`.
Tudo **grátis e open source (MIT)**, com doação: PIX `pix@xwa.com.br` e GitHub Sponsors (`lordironballs`, aprovação pendente).
Créditos: Eddy Antonini — eddyantonini.com.br.

| Produto | Repositório | Situação |
| --- | --- | --- |
| Backeddy for Mac (ex-Force TM) | BackEddy/mac | 2.0.0 publicada, notarizada |
| Backeddy Server | BackEddy/server | 0.3.0: instalador do pendrive escrito, aguardando 1º build |
| Backeddy for Windows | (BackEddy/windows, futuro) | Especificado, não iniciado |

## Backeddy for Mac (ex-Force TM)

- App SwiftUI que força, acompanha e verifica backups do **Time Machine**; limpa snapshots locais,
  mídia do WhatsApp e caches; modo turbo; barra de menus; histórico; ajuda; PT/EN/ES.
- Projeto em `~/tm-helper` (pasta antiga, ainda com esse nome). `./build.sh` compila e instala em
  ~/Applications; `./release.sh` assina com Developer ID, notariza (perfil `force-tm-notary`) e gera o DMG.
- Renomeado em 16/10/2026: bundle **br.com.backeddy.mac**, versão **2.0.0**, migra histórico/log/ajustes
  do Force TM na primeira abertura. Repo transferido `lordironballs/force-tm` → `BackEddy/mac` (redirects ok).
- Fora da Mac App Store por causa do sandbox (decisão aceita).
- Pendente: ícone e screenshot com a marca Backeddy (ainda são do Force TM).
- Futuro (2.1): aba **Servidor** (mede o Mac, recomenda disco, grava o pendrive do Backeddy Server,
  detecta o servidor e configura o Time Machine com um clique).

## Backeddy for Windows (especificado)

- "Time Machine para Windows". **C# + WinUI 3**, motor **Kopia** (Apache 2.0) embutido como executável
  com versão fixa; interface e serviço próprios.
- Princípios: worry-free (tudo entra, usuário só exclui), setup anti-burro, só peças incorporáveis,
  APIs de base do Windows permitidas (VSS, registro, WLAN, SetupAPI), nada de recursos que a Microsoft
  pode descontinuar (Histórico de Arquivos, wbadmin) nem apps de terceiros não embutíveis (Veeam, PCmover).
- Inclui: categorias (Arquivos, Aplicativos, Jogos, Configurações, Outros discos), saves de jogos (Ludusavi),
  migração inteligente (inventário + colmeias do registro + receitas + catálogo winget-pkgs como dados),
  imagem do sistema (VSS + Kopia) e pendrive de resgate Linux (partclone), lições do Duplicati.
- Precisa de um PC Windows para desenvolver/testar. Detalhes completos na especificação.

## Backeddy Server

### Produto (visão)
Pendrive criado pelo app Backeddy ("Criar pendrive") que transforma qualquer PC largado num servidor de
backup para Macs (Time Machine/Samba) e PCs (Kopia Server). "Espeta, confirma, pronto."
Decisões: espelhar por padrão com 2 discos iguais; Raspberry Pi só numa versão futura; HD externo permitido
com aviso; pendrive só pelo app (por enquanto); arquivo de plano no pendrive (tamanho mínimo/recomendado,
computadores, nome, idioma, fuso, Wi-Fi criptografado opcional) com verde/amarelo/vermelho na instalação
(vermelho com duas barreiras: tela com números + digitar CONTINUAR); perguntas na tela do PC (celular depois,
via QR); x86 64 bits apenas (Debian 13 sem i386; Kopia sem binário 32 bits).

### Estado técnico (0.2.1, em ~/backeddy-server)
- `install.sh`: Debian 12/13 limpo → Samba afinado para Time Machine (vfs fruit, durable handles),
  Avahi (`_adisk`, aparece sozinho no Mac), Kopia Server porta 51515 (TLS próprio, sem UI, usuário por PC),
  btrfs, nunca dorme, atualizações automáticas, smartd. Idempotente.
- Discos (0.2): `--disk` repetível; 2+ discos = espelho btrfs RAID1 (`--no-mirror` soma); aviso de tamanhos
  diferentes; montagem pelo serviço `backeddy-mount` (sem fstab; modo degradado se faltar disco);
  Samba/Kopia só sobem com os discos montados; ponto de montagem imutável.
- `backeddy status | add-pc | passwd | replace-disk | add-disk`.
- Testes: `docs/TESTING.pt-BR.md` (instalação) e `docs/TESTING-discos.pt-BR.md` (espelho, defeito, troca).

### Histórico do teste (16/10/2026)
- VM Hyper-V "Backeddy-Server" no PC "ANTONINAS": Geração 2, 2 vCPU, **512 MB** (PC só tinha ~1,1 GB livre),
  switch `Bridge_Rede`, sistema 16 GB, backup `B:\Backeddy\backups.vhdx` 1 TB dinâmico. Debian 13.4 netinst
  em low memory mode. IP 192.168.0.67, usuário `eddy`.
- Convive com a VM antiga "TimeMachine" (512 MB), que continua sendo o Time Machine de verdade.
- 0.1 instalada: tudo verde, **Mac encontrou "Backup da Casa" na lista do Time Machine**.
- Bug corrigido: `/etc/os-release` sobrescrevia NAME/VERSION (servidor saiu "Debian GNU/Linux");
  agora lido em subshell; Kopia maintenance owner e certificado acompanham renomeações.
- Eddy optou por **não fazer backup de teste** agora ("na cega", testa depois).
- **Testes de disco (0.2.0) concluídos com sucesso**: atualização + reboot; espelho 20 GB + 30 GB (aviso de
  tamanhos diferentes); disco removido → servidor sobe degradado (~30–40 s a mais) e avisa; disco novo de
  30 GB com `backeddy replace-disk` → espelho reconstruído; reboot final tudo verde; scrub sem erros.
- Bugs achados e corrigidos na 0.2.1:
  1. Cache do Kopia (`/var/cache/backeddy-kopia`, inclusive o blob de formato guardado por 15 min) de um
     repositório antigo fez o servidor gravar blobs com a chave errada no repositório novo
     ("cipher: message authentication failed"). Agora o cache é apagado sempre que um repositório é criado
     ou conectado. Na VM foi resolvido recriando o repositório (estava vazio).
  2. `backeddy status` quebrava (set -e + du) e mostrava o disco do sistema quando os discos não estavam montados.
  3. Status mostra ⏳ "aguardando os discos" enquanto o boot espera um disco que sumiu.
  4. Limite do Time Machine agora usa o espaço útil do btrfs (dados + livre estimado), não o `df`.
  5. Roteiros: cópia para a VM com `tar … | ssh` (o `scp -r` cria pasta dentro de pasta e erra no .git);
     PowerShell de criação com checkpoints automáticos desligados (o Hyper-V do Windows 11 cria checkpoints
     e os discos viram `.avhdx`, o que também quebrou o filtro por caminho); remover discos pela posição.
- Estado atual da VM: espelho com disco-a (20 GB) + disco-c (30 GB); `backups.vhdx` (1 TB) desanexado, ainda
  no B:; checkpoints automáticos desligados. A VM antiga "TimeMachine" pode ter o mesmo problema de checkpoints
  (verificar `Get-VMCheckpoint TimeMachine`).
- 0.3.0 publicada (commits df60cdc + b24a1ba: `-buildvcs=false` no go build do CI).

### Fase 2 — instalador do pendrive (0.3.0, escrita em 16/10/2026 ~23h50)
- Decisões: só cabo de rede; sem testes automáticos de boot (manual no Hyper-V); só UEFI (BIOS antiga depois).
- `installer/` (Go, sem dependências; testes de receitas/plano/dry-run passam): telas grandes (setas/Enter),
  receitas de discos (1 disco = compartilhado; SSD/disco pequeno + outros = sistema dedicado; discos parecidos =
  parte do primeiro + espelho; ignora o próprio pendrive; avisa USB), plano verde/amarelo/vermelho com duas
  barreiras, APAGAR, nome e senha, checagem de cabo. Particiona (GPT: EFI 512M + root 20G [+ dados]; discos de
  backup com 1 partição `backeddy-data` de PARTUUID conhecido), copia `rootfs.tar.zst`, cria usuário `admin`
  (senha do servidor), recria chaves SSH/machine-id, grub-install (removível, shim assinado) + update-grub.
- Primeiro boot: `backeddy-firstboot` roda o mesmo `install.sh` com `--yes` e os discos por PARTUUID; apaga
  `/etc/backeddy/firstboot.env` (tem a senha); atualiza `/etc/issue` com endereço e instruções.
- `install.sh` 0.3.0 funciona offline quando os pacotes já vêm na imagem.
- `image/build.sh`: mmdebstrap (Debian 13) monta o sistema do servidor (tar.zst, com Kopia) e o do instalador
  (autologin no tty1 → instalador, fonte Terminus 32); ESP com shim/grub assinados copiados; sistemas de arquivos
  criados de diretórios (mkfs.ext4 -d, mtools), sem loop; saída .img.xz (Etcher), .vhdx (Hyper-V), SHA256SUMS.
- Workflow `.github/workflows/image.yml` (container debian:trixie --privileged), manual ou por tag v*.
- Para dar push em arquivos de workflow, o gh precisa do escopo `workflow` (`gh auth refresh -s workflow`).

### Teste da 0.3.0 no Hyper-V (17/10/2026) — PASSOU
- Build do CI: run 39034235713, ~5 min, artefato `backeddy-server-imagem` (1,44 GB; baixar logado no GitHub).
- VM "Backeddy-Teste-Pendrive" (1 GB, B:\Backeddy\teste-pendrive): pendrive .vhdx + ssd 24G + hd1/hd2 60G.
- Secure Boot ok, instalador direto; receita escolhida: SSD 24G só sistema + espelho 2×60G; pendrive fora da lista.
- Instalou em ~2 min; tirar o "pendrive" com Stop-VM/Remove-VMHardDiskDrive; primeiro boot ok;
  status tudo verde ("Espelho de 2 discos", 59 GB livres), IP 192.168.0.68; o Mac encontra "Backup da Casa".
- Defeitos visuais achados: emojis viram "◆" no console; mensagens "systemd-ssh-generator ... AF_VSOCK"
  (só Hyper-V) sujando a tela; título da boas-vindas passando da borda; fonte grande demais no servidor.

### 0.4.0 — instalador gráfico + tela do servidor (escrita em 17/10/2026, falta build e teste)
- Pedidos do Eddy: fonte padrão no servidor; fim do setup "Retire o pendrive e aperte Enter" + aviso de
  mensagens no primeiro início; boot sempre termina com "servidor funcionando", diagnóstico completo e
  endereço SSH; instalador mais gráfico, com cara de Mac (opção B, só teclado); reiniciar ao tirar o pendrive.
- Instalador: `gui.go`/`draw.go`/`fb.go` desenham no /dev/fb0 (janela clara estilo macOS, etapas na lateral,
  botões à direita, spinner, fonte Inter embutida); console em KD_GRAPHICS; cai no modo texto se não houver
  framebuffer (`--text` força). Dependências vendorizadas (x/image, x/text) para o build não precisar de rede.
  `--shots DIR` gera PNGs das telas. Sem identidade da Apple: sem os botões "semáforo" de janela e com
  ícone próprio (degradê laranja→rosa da família, dois discos empilhados + seta), não um relógio com seta.
  Funciona de 640×480 para cima (abaixo disso, modo texto). Roda de /run (RAM) para o pendrive poder sair: ao sumir a partição
  `backeddy-inst`, mostra "Reiniciando…" e reinicia (syscall.Reboot).
- Servidor: console Uni2-Fixed16 (padrão), GRUB `quiet loglevel=3 systemd.ssh_auto=no` (sem VSOCK);
  primeiro boot com tela limpa no tty1 (spinner + tempo); `backeddy issue` + `backeddy-issue.service/.timer`
  (boot e a cada 2 min) escrevem /etc/issue com "Nome — servidor funcionando/atenção/com problema", Mac,
  Windows, SSH (IP via `\4` do agetty) e o `backeddy status` completo; getty@tty1 espera firstboot e issue.
  No console, `backeddy status` usa [  OK  ]/[AVISO ]/[ ERRO ] coloridos; por SSH continua com emojis.

### 0.4.0 testada (17/10/2026) e 0.5.0 escrita
- 0.4.0 aprovada no Hyper-V: instalador gráfico, reinício ao tirar o pendrive, tela do servidor com SSH.
  Bug achado: /run é noexec no Debian 13 → instalador agora roda de um tmpfs próprio (/run/backeddy-bin)
  e faz mlockall. A "interface web" do Windows não existe (Kopia com --no-ui): texto da tela corrigido.
- 0.5.0 (spec "Discos e modos" no Claude Docs): mouse (evdev, relativo/absoluto/touchpad, cursor, hover);
  cartões de modo Protegido / Mais espaço / Tudo junto / Um disco só, com espaço e risco (verde/amarelo/vermelho;
  amarelo = tela de aviso, vermelho = digitar CONTINUAR); Manual (papel por disco + chave do espelho);
  SMART no instalador (disco ruim fica de fora; no Manual vira vermelho); avisos "tem Windows/Linux/backups";
  "Quem vai fazer backup aqui?" sem plano; Recuperar servidor (install.sh --recover; configuração guardada
  em /srv/backeddy/.backeddy: server.conf, senhas e certificado do Kopia; reaproveita as partições do sistema
  se o disco antigo existir); `backeddy mirror`/`unmirror` + recálculo do limite do Time Machine.
  Testes: matriz de discos, Manual, recuperação, SMART, lsblk e roteiros completos do instalador (flow_test.go).

### 0.6.0 (17/10/2026, escrita em passos curtos, falta build e teste)
- Passo a passo combinado com o Eddy (tokens curtos): 1 APAGAR no campo; 2 divisão automática do espaço
  (backeddy space + timer de hora em hora: TM = útil − Kopia − folga 10%/20 GB, piso 20%; sem a tela
  "Quem vai fazer backup"); 3 vigia em Go (agent/: backeddy check a cada minuto, bipe pcspkr 3x/min,
  qualquer tecla no servidor reconhece por 1 dia, backeddy ack/alerts/quiet, silêncio 22–8);
  4 painel web na porta 80 (senha = hash NT do Samba via pdbedit; reconhecer 1/3/7 dias; silêncio;
  Bonjour _backeddy._tcp); 5 teste do alarme + silêncio no instalador; 6 imagem pronta de VM no CI
  (-vm.vhdx e -vm.vdi; disquinho FAT BACKEDDYCFG com firstboot.env; DISKS=auto; admin e chaves SSH no
  1º boot; hyperv-daemons para o IP); 7 windows/Instalar-Backeddy.ps1 (checa BIOS, Hyper-V liga com 1
  reinício ou cai no VirtualBox; uma janela: nome, senha, unidade, tamanho, silêncio; cria switch em
  ponte se faltar; VM 512 MB, Gen2, Secure Boot, liga/desliga com o Windows); 8 caminho VirtualBox
  (instala VC++ e VirtualBox em silêncio, ponte, SATA, tarefa agendada para ligar; limitação: Windows
  desligando corta a VM); 9 vigia na bandeja do Windows (/api/beep sem senha, bipa e avisa).
- Decisões: descartado "Faça tudo por mim"; teste de backup real e de hardware com um amigo que já tem
  servidor Windows (usa o instalador de VM).

### 0.6.1 — VM no Windows pelo "pendrive virtual" (17/10/2026)
- Teste da 0.6.0 no Hyper-V: a VM com imagem pronta configurou o servidor (o Mac achou), mas o 1º boot
  não terminou (causa não vista: o registro foi sobrescrito no reinício), o SSH ficou fora (sem chaves no
  boot → start-limit) e o Windows não recebeu o IP (KVP). Ideia do Eddy, adotada: a VM dá partida pelo
  pendrive virtual (mesmo instalador gráfico já testado) e uma janelinha no Windows tem "Ejetar pendrive"
  (Hyper-V: Remove-VMHardDiskDrive com a VM ligada; VirtualBox: porta SATA hotpluggable + medium none).
- Saiu a imagem pronta de VM e o disquinho de configuração; o CI gera o pendrive também em .vdi.
- Instalador gráfico reconhece VM (DMI): cartão verde "Disco de backup da máquina virtual" e sem teste de
  bipe (pergunta só o silêncio à noite). /api/hello (sem senha) devolve nome/slug para o Windows.
- O Windows acha a VM pelo IP do Hyper-V ou pelo MAC na tabela ARP (com "ping" na rede /24).

### 0.6.2 — correções do teste da 0.6.1 (17/10/2026)
- Disco do sistema de 20 GB não cabia (EFI 512M + root 20G fixo): root agora ocupa o disco inteiro quando o
  disco é só do sistema; o instalador do Windows cria 24 GB.
- Primeiro boot travava com 0% de CPU (também na VM de imagem pronta): impasse do systemd — o install.sh
  esperava "systemctl restart backeddy-agent", e o vigia tinha After=multi-user.target, que só chega depois
  do primeiro boot. Vigia agora After=network.target e o install.sh liga serviços novos com --no-block.
- Próxima rodada: silêncio à noite com horário digitado (instalador e painel); .exe único; releases.

### 0.6.3 — .exe único e acabamentos (17/10/2026)
- 0.6.2 aprovada no Hyper-V pelo instalador do Windows (VM pelo pendrive virtual, Ejetar, painel, vigia, Mac vê o TM).
- Vigia iniciado com conhost --headless (no Windows 11 o -WindowStyle Hidden deixava um Terminal vazio).
- Status na VM: "Disco da máquina virtual" em verde (systemd-detect-virt), sem o aviso de sem espelho.
- Silêncio à noite: cartão "Escolher o horário" (digita início e fim, 0–23).
- CI: job "windows" (windows-latest) embute o vigia no script e gera Instalar-Backeddy.exe com ps2exe
  (-noConsole -requireAdmin); artefato backeddy-windows; em tag v*, release com .exe + pendrive (.vhdx/.vdi/.img.xz).
  O .exe baixa sozinho o pendrive virtual da última release. Sem assinatura ainda (SmartScreen avisa).

### Plano de construção
| Fase | Conteúdo | Situação |
| --- | --- | --- |
| 0 | Repo BackEddy/server e forma de publicar | Feita |
| 1 | Script do servidor + VM | Feita (0.1); 0.2/0.2.1 discos testados |
| 2 | Instalador em Go + imagem do pendrive (.img com partição gravável do plano) | Feita (0.3.0 testada); 0.4.0 gráfica escrita |
| 3 | Painel (monitor + web), **alertas**, arquivo de plano, divisão de espaço | A fazer |
| 4 | GitHub Actions: mmdebstrap/live-build, testes QEMU, assinatura, Releases | A fazer |
| 5 | Aba Servidor no Backeddy for Mac 2.1 | A fazer |
| 6 | Hardware real e lançamento | A fazer |

Requisitos: x86 64 bits; 512 MB–1 GB (recomendado 2 GB); sistema 16 GB; rede Gigabit recomendada;
HDs CMR (evitar SMR); notebook é ótimo (8–15 W, bateria = nobreak).

## Pendências gerais
- Teste real de backup do Time Machine e de um PC no Kopia (ainda não feitos, por opção).
- Alertas do servidor (lacuna conhecida).
- Ícone/screenshot Backeddy; redirecionamento de backeddy.com.br; GitHub Sponsors (aguardando).
- Divulgação planejada: r/macapps (precisa de 10 karma local, formato PCP, prefixo [OS], site com
  Política de Privacidade e Termos para o caminho "transparência"), TabNews, Show HN, Product Hunt,
  awesome-mac, AlternativeTo, Homebrew Cask (depois de ter estrelas), vídeo no canal do Eddy.
