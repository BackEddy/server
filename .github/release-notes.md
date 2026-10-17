**Mac user?** The easiest way to set up a server is the [Backeddy for Mac](https://github.com/BackEddy/mac#readme) app: it writes the USB drive for you and explains everything. · Usa Mac? O jeito mais fácil é o app [Backeddy for Mac](https://github.com/BackEddy/mac/blob/main/README.pt-BR.md). · ¿Usas Mac? Lo más fácil es la app [Backeddy for Mac](https://github.com/BackEddy/mac/blob/main/README.es.md).

| File | What it's for |
| --- | --- |
| `Instalar-Backeddy.exe` | Windows PC that's always on: creates a Backeddy Server virtual machine, nothing gets erased |
| `backeddy-server-*.img.xz` | USB drive for a PC just for backups (write it with Backeddy for Mac or balenaEtcher) |
| `backeddy-server-*.vhdx` / `.vdi` | The same USB drive as a virtual disk (Hyper-V / VirtualBox), used by the Windows installer |
| `SHA256SUMS` | Checksums |

### ⚠️ "Windows protected your PC"

`Instalar-Backeddy.exe` **isn't digitally signed yet** (we're working on it), so Windows SmartScreen warns about it. To run it: **More info › Run anyway**. The installer is open source — you can read every line in [`windows/`](https://github.com/BackEddy/server/tree/main/windows) — and it's built automatically by GitHub Actions from that code.

**PT** — O `Instalar-Backeddy.exe` **ainda não tem assinatura digital** (estamos providenciando), por isso o Windows avisa “O Windows protegeu o computador”. Para rodar: **Mais informações › Executar assim mesmo**. O código é aberto e o arquivo é montado automaticamente pelo GitHub a partir dele.

**ES** — `Instalar-Backeddy.exe` **todavía no tiene firma digital** (estamos en ello), por eso Windows avisa “Windows protegió su PC”. Para ejecutarlo: **Más información › Ejecutar de todas formas**. El código es abierto y GitHub genera el archivo automáticamente a partir de él.

Setup, the server screen, alerts and the web panel are available in English, Português and Español.
