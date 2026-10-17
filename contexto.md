# Contexto do projeto Backeddy

> Arquivo de memória do projeto: tudo o que foi decidido e feito nas conversas com o Claude.
> Leia antes de continuar o trabalho. Atualizado em 16/10/2026 (noite).

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
| Backeddy Server | BackEddy/server | 0.2 (script) em teste |
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

### Estado técnico (0.2, em ~/backeddy-server)
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
- Próximo: copiar a 0.2 para a VM e seguir `docs/TESTING-discos.pt-BR.md` (inclui teste de reboot).

### Plano de construção
| Fase | Conteúdo | Situação |
| --- | --- | --- |
| 0 | Repo BackEddy/server e forma de publicar | Feita |
| 1 | Script do servidor + VM | Feita (0.1); 0.2 discos em teste |
| 2 | Instalador em Go + imagem do pendrive (.img com partição gravável do plano) | A fazer |
| 3 | Painel (monitor + web), **alertas**, arquivo de plano, divisão de espaço | A fazer |
| 4 | GitHub Actions: mmdebstrap/live-build, testes QEMU, assinatura, Releases | A fazer |
| 5 | Aba Servidor no Backeddy for Mac 2.1 | A fazer |
| 6 | Hardware real e lançamento | A fazer |

Requisitos: x86 64 bits; 512 MB–1 GB (recomendado 2 GB); sistema 16 GB; rede Gigabit recomendada;
HDs CMR (evitar SMR); notebook é ótimo (8–15 W, bateria = nobreak).

## Pendências gerais
- Testes de disco da 0.2 na VM; depois teste real de backup do Time Machine e de um PC no Kopia.
- Alertas do servidor (lacuna conhecida).
- Ícone/screenshot Backeddy; redirecionamento de backeddy.com.br; GitHub Sponsors (aguardando).
- Divulgação planejada: r/macapps (precisa de 10 karma local, formato PCP, prefixo [OS], site com
  Política de Privacidade e Termos para o caminho "transparência"), TabNews, Show HN, Product Hunt,
  awesome-mac, AlternativeTo, Homebrew Cask (depois de ter estrelas), vídeo no canal do Eddy.
