# DualCenter v1.0 Oficial — Build 30

Aplicativo residente para Windows que transforma o botão PS do DualSense e do DualSense Edge em um centro rápido para bateria, volume, energia, jogos e configurações.

## Requisitos

- Windows 10 ou Windows 11 de 64 bits.
- Controle DualSense ou DualSense Edge por USB ou Bluetooth.
- Permissão de administrador somente durante a instalação ou remoção em `Arquivos de Programas`.

O aplicativo instalado executa com as permissões normais do usuário.

## Controles principais

- Um toque no botão PS: exibe a bateria, se o overlay de bateria estiver habilitado.
- Dois toques no botão PS: abre ou fecha o centro do DualCenter.
- Segurar o botão PS por três segundos: solicita o desligamento, exceto quando o bloqueio está ligado.
- Direcional: navega entre abas e opções.
- X: seleciona ou confirma.
- Círculo: volta ou fecha. Ao fechar o centro, o foco só retorna ao jogo depois da soltura física do botão, evitando que o mesmo comando seja recebido pelo jogo.
- Triângulo: organiza as abas ou abre as ações de um jogo.

Em tela cheia exclusiva, o Windows pode impedir overlays comuns. Nessa situação, o DualCenter orienta o uso de tela cheia sem borda ou modo janela.

## Abas e opções

- `Energia`: desligar, reiniciar, suspender ou cancelar um desligamento pendente.
- `Preferências`: ligar ou desligar o `Overlay Bateria`, ativar o aviso de bateria, bloquear o desligamento pelo botão PS e mostrar ou ocultar o ícone na barra de tarefas.
- `Sistema`: iniciar com o Windows, ligar ou desligar a Game Bar, escolher o tempo do aviso e consultar a versão.
- `Volume` e `Controle`: mantêm seus blocos principais grandes; as duas ações informativas ou interativas ficam alinhadas nas últimas posições de cada painel.

O símbolo Xbox identifica a Game Bar, a bandeira do Windows identifica a integração com a barra de tarefas e o símbolo USB identifica a conexão do controle. Os ícones Xbox e USB usam as artes fornecidas, em branco e com a mesma escala dos demais ícones.

O centro usa uma sombra preta em degradê atrás dos cartões grafite, títulos em Century Gothic Semibold, textos em Segoe UI Variable e seleção azul de baixo brilho. Ícones e textos seguem a mesma grade visual; ao mudar o foco, uma transição curta de 120 ms é executada e encerrada automaticamente para preservar o baixo consumo.

O ícone residente vem ligado por padrão. O clique esquerdo e o clique duplo não executam nada; o clique direito mostra somente `Fechar`, pois o centro foi projetado para ser aberto pelo controle.

Enquanto o menu está aberto, o DualCenter bloqueia temporariamente a entrada física de mouse e teclado, além de ocultar e prender o cursor ao overlay. Isso impede que jogos com Raw Input movimentem a câmera ao fundo. A posição anterior e o uso normal da entrada são restaurados somente depois que o overlay fecha e o foco volta ao jogo.

Durante a instalação, o usuário escolhe se a Game Bar deve permanecer ligada. A preferência é aplicada na primeira abertura do DualCenter sem exibir uma pergunta adicional. O valor anterior do Windows é preservado e restaurado durante a desinstalação.

## Instalação e remoção

Execute `DualCenter-Setup-v1.0-Oficial.exe`. O instalador compacto permite escolher o destino, os atalhos, a abertura automática e o comportamento da Xbox Game Bar. Ele não aceita instalar na raiz da unidade ou na pasta do Windows e não mistura o aplicativo com uma pasta que contenha arquivos desconhecidos.

O desinstalador usa o mesmo padrão visual e a mesma tela de progresso do instalador. Ele remove somente os arquivos registrados; arquivos adicionais encontrados na pasta são preservados. Antes de sair, restaura a configuração anterior da Xbox Game Bar e oferece a escolha de manter ou apagar configurações e logs.

## Dados locais e privacidade

O DualCenter funciona localmente, sem telemetria e sem conta. Os arquivos do usuário ficam em:

```text
%LOCALAPPDATA%\DualCenter\settings.json
%LOCALAPPDATA%\DualCenter\DualCenter.log
```

O log é limitado a aproximadamente 1 MiB e mantém somente um backup. Consulte [PRIVACY.md](PRIVACY.md) e [SECURITY.md](SECURITY.md).

## Build

Requer Go 1.23 ou mais recente no Windows:

```powershell
.\build.ps1 -SkipSigning
```

ou:

```bat
build.cmd -SkipSigning
```

O build gera os recursos do Windows a partir de `internal/version/version.json`, verifica formatação, executa `go vet` e os testes, compila o aplicativo, incorpora o payload no instalador e produz o SHA-256 em `dist`.

Para assinatura Authenticode, instale o Windows SDK e defina:

```powershell
$env:DUALCENTER_SIGN_CERT_SHA1 = "IMPRESSAO_DIGITAL_DO_CERTIFICADO"
$env:DUALCENTER_TIMESTAMP_URL = "https://servidor-de-timestamp"
.\build.ps1
```

Sem essas variáveis, o build avisa que os executáveis não foram assinados. O pipeline de CI usa `-SkipSigning` porque certificados privados não devem ficar no repositório.

## Estrutura técnica

- `main.go`: inicialização, loop de mensagens e ciclo de vida da janela.
- `controller_windows.go` e `menu_input_windows.go`: Raw Input do DualSense e navegação do centro.
- `games_windows.go` e `audio_windows.go`: biblioteca de jogos e integração com áudio do Windows.
- `overlay_window_windows.go` e `overlay_runtime_windows.go`: foco, cursor, posicionamento, timers e transições.
- `overlay_render_windows.go`, `overlay_surfaces_windows.go` e `overlay_theme_windows.go`: pipeline gráfico, superfícies e métricas visuais.
- `overlay_panels_windows.go`, `overlay_cards_windows.go` e `overlay_assets_windows.go`: componentes da interface, cartões e recursos incorporados.
- `platform_windows.go`, `app_state_windows.go` e `win32_api_windows.go`: tipos nativos, estado central e APIs Win32.
- `power_windows.go`, `gamebar_windows.go`, `taskbar_windows.go` e `startup_windows.go`: integrações do sistema.
- `settings_windows.go`, `file_storage.go` e `log_maintenance.go`: persistência, recuperação e manutenção do log.
- `installer/`: plataforma, interface, instalação/remoção protegida e atalhos separados por responsabilidade.
- `cmd/resourcegen/` e `internal/version/`: recursos do executável e fonte única de versão.

## Licença

Distribuído sob a licença MIT. Consulte [LICENSE](LICENSE).
