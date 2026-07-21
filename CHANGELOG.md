# Changelog

## v1.0 Oficial - Build 30 - Navegação imediata e renderização 4K otimizada

- A navegação do controle não renderiza mais o overlay de forma síncrona dentro do processamento de Raw Input; o frame é composto pelo `wndProc` depois que o comando termina.
- Solturas de X, Círculo e Triângulo e o retorno do direcional ao centro deixaram de solicitar frames sem mudança visual.
- Removida a transição de foco de 120 ms: a seleção agora acompanha o comando no primeiro frame e o menu não sobe mais para 60 Hz durante a navegação.
- A limpeza da superfície DIB usa o caminho otimizado do runtime e a composição de artes recorta os limites uma única vez, eliminando verificações e conversões de ponto flutuante por pixel.
- A geração inicial de sombras, painéis e molduras ajusta o supersampling à densidade da tela, preservando a suavidade física e reduzindo fortemente a demora da primeira abertura em 4K.
- O visual, as dimensões, as cores e a tipografia aprovadas foram preservados.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.30`.

## v1.0 Oficial - Build 29 - Candidata final pública

- O menu agora usa `BlockInput` enquanto está aberto, impedindo que a entrada física do mouse chegue a jogos que leem movimento por Raw Input.
- O bloqueio permanece ativo durante o fechamento e só é liberado depois que cursor, janela e foco foram restaurados, evitando movimento residual no jogo.
- O seletor de executáveis libera a entrada para uso normal e a bloqueia novamente ao retornar ao menu, inclusive no caminho de erro ao abrir o diálogo.
- O aviso de tela cheia exclusiva mantém cores, fontes, sombra e posição, mas agora centraliza título e instrução e aproxima as bordas do texto.
- A restauração do overlay foi centralizada em uma única rotina; a função intermediária de cursor que ficou sem chamadas foi removida.
- As duas conversões UTF-16 obsoletas do aplicativo e do instalador foram substituídas pela API atual, com tratamento seguro de erro.
- Imagens e recursos incorporados foram inventariados; todos os arquivos atuais possuem uso confirmado e nenhum recurso visual ativo foi descartado.
- A versão pública foi definida como `DualCenter v1.0 Oficial`; a versão de arquivo do Windows e o VERSIONINFO usam `1.0.0.29`.

## v1.0 Oficial - Build 28 - Tipografia e identidade visual refinadas

- O novo ícone oficial do DualCenter foi aplicado ao programa, instalador, desinstalador, barra de tarefas, bandeja do sistema, Apps e Recursos e atalhos do Menu Iniciar e da Área de Trabalho.
- O arquivo `.ico` antigo foi substituído por um único recurso multirresolução, com imagens nativas de 16 a 256 pixels geradas a partir da nova arte em PNG.
- O instalador agora avisa o Explorer após recriar cada atalho, evitando que a Área de Trabalho ou o Menu Iniciar mantenham o ícone anterior em cache.
- O instalador antigo foi substituído pela interface compacta de 660×400, com cabeçalho integrado à barra de título e conteúdo sem moldura.
- Adicionada a escolha de manter ou desativar a Xbox Game Bar antes da instalação, sem pergunta adicional na primeira execução.
- O desinstalador agora usa o mesmo padrão visual, oferece a limpeza opcional de configurações e logs e exibe o progresso da remoção.
- Toda a tipografia do instalador e do desinstalador usa Segoe UI regular com peso normal e renderização ClearType para manter os textos nítidos.
- Removidos o fluxo de desinstalação por caixas de diálogo e as constantes antigas que deixaram de ser utilizadas.
- A Bahnschrift foi substituída por Century Gothic Semibold nos títulos das abas.
- A nova tipografia deixa os títulos mais geométricos, elegantes e profissionais, mantendo os textos das opções em Segoe UI Variable para preservar a leitura.
- Enquanto o menu está aberto, o DualCenter agora captura as mensagens do mouse antes de reposicioná-lo e reaplica a captura caso outro programa tente tomá-la.
- A captura é liberada ao fechar o menu ou abrir o seletor de jogos, devolvendo o mouse normalmente ao Windows.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.28`.

## v1.0 Oficial - Build 27 - Cursor e preferência da bateria

- O cursor agora é ocultado novamente quando o Windows ou um jogo torna a seta visível durante o uso do menu.
- O contador de `ShowCursor` continua sendo restaurado pelo número exato de chamadas feitas pelo DualCenter ao fechar o overlay.
- A opção `Ocultar overlay de bateria` foi renomeada para `Overlay Bateria`.
- O estado agora usa semântica direta: `Ligado` exibe o overlay e `Desligado` não exibe.
- Os títulos das abas agora usam Bahnschrift Semibold, com aparência mais técnica, premium e consistente com o Windows 10/11.
- Adicionado teste para impedir que essa semântica seja invertida em mudanças futuras.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.27`.

## v1.0 Oficial - Build 26 - Limpeza e otimização final

- Lotes Raw Input maiores que o buffer rápido agora usam uma leitura dinâmica limitada, evitando perda de comandos quando o Windows agrupa vários relatórios HID.
- A varredura de processos da aba Jogos saiu da thread da interface, mantendo PS, direcional, X e Círculo responsivos durante a consulta dos executáveis ativos.
- Resultados de varreduras antigas são descartados quando a biblioteca de jogos muda durante a consulta.
- Removida uma goroutine redundante ao acompanhar jogos iniciados pelo DualCenter.
- A gravação do instalador passou a sincronizar os arquivos no disco e a substituir resíduos temporários com criação exclusiva.
- O publicador do instalador agora vem da mesma fonte central usada pelos metadados de versão.
- O build remove automaticamente payload e recursos `.syso` gerados, impedindo artefatos obsoletos no pacote do código-fonte.
- Removida uma constante visual sem uso; testes adicionados para os novos limites e proteções.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.26`.

## v1.0 Oficial - Build 25 - Organização e robustez técnica

- O núcleo de 5.773 linhas foi separado em módulos de plataforma, estado, controle, navegação, jogos, áudio, janela, runtime, renderização, superfícies, painéis, cartões e recursos.
- O instalador de 1.136 linhas foi dividido entre plataforma Win32, instalação e remoção, interface e ponto de entrada.
- Métricas de escala, espessura, cantos e cores Win32 foram reunidas em uma base visual única, sem alterar o visual aprovado.
- Testes do aplicativo foram organizados por controle, configurações e interface.
- Corrigida uma possibilidade de overflow nos metadados de lotes Raw Input que poderia causar panic ao recortar um relatório HID inválido.
- O seletor de áudio agora normaliza uma seleção obsoleta após mudanças na lista de dispositivos, evitando acesso fora dos limites.
- Adicionados testes para as duas proteções e mantidas as validações de cursor, overlay neutro, espaçamento, ícones e transição.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.25`.

## v1.0 Oficial - Build 24 - Refinamento visual profissional

- Adicionada uma base única de vidro escuro para integrar visualmente os cinco cartões sem esconder o jogo.
- Os cartões deixaram o preto absoluto e passaram a usar grafite com profundidade, bordas neutras e mais espaço entre as abas.
- Títulos agora usam Segoe UI Variable Display Semibold; opções e estados usam Segoe UI Variable Text com hierarquia e legibilidade melhores.
- O neon foi refinado com linha mais precisa, halo reduzido e preenchimento azul discreto.
- Ícones Fluent, Xbox, USB e Windows foram padronizados em tamanho, alinhamento e cor; somente o item ativo recebe azul.
- Adicionada transição ease-out de 120 ms ao mudar o foco. O timer sobe para 60 Hz apenas durante a transição e retorna a 10 Hz em seguida.
- Removidos todos os tons azuis do aviso `Overlay indisponível`.
- Fundo, brilho, borda, faixa lateral e ícone agora usam somente grafite, cinza e branco.
- Mantidos o tamanho, a posição, a tipografia e a legibilidade do cartão sobre jogos claros ou escuros.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.24`.

## v1.0 Oficial - Build 23 - Cursor isolado do jogo

- O cursor agora é reposicionado somente depois que o menu está visível e em primeiro plano, impedindo que o jogo receba o movimento atrás do overlay.
- A seta continua oculta durante todo o uso do menu e sua posição original é restaurada ao fechar.
- O contador de visibilidade do Windows passou a ser restaurado exatamente, sem acumular chamadas quando um jogo tenta mostrar o cursor novamente.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.23`.

## v1.0 Oficial - Build 22 - Ícones da conexão e Game Bar

- A opção `Xbox Game Bar` foi renomeada para `Game Bar` na interface.
- O `X` textual da Game Bar foi substituído pela arte Xbox fornecida.
- O glifo genérico da conexão foi substituído pela arte USB fornecida.
- As duas artes são usadas como máscaras transparentes, renderizadas em branco e dimensionadas na mesma caixa visual dos demais ícones.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.22`.

## v1.0 Oficial - Build 21 - Organização, controle e estabilidade

- As abas `Modo jogo` e `Configurações` passaram a se chamar `Preferências` e `Sistema`.
- A Xbox Game Bar foi movida para `Sistema`; o ícone da barra de tarefas permaneceu em `Preferências`.
- A ação `Encerrar DualCenter` foi removida do overlay. O encerramento completo agora fica somente no menu `Fechar` aberto com o botão direito do ícone residente.
- Cliques esquerdo e duplo no ícone residente não executam nenhuma ação, preservando o controle como única forma de abrir o centro.
- Adicionada a ação `Suspender computador` à aba Energia por meio da API nativa do Windows.
- Todos os retângulos internos de ação foram padronizados. Volume e Controle mantêm seus blocos principais grandes e alinham suas duas linhas nas últimas posições.
- A conexão do controle passou a usar o símbolo USB; a Xbox Game Bar usa um `X` direto e o ícone residente usa a bandeira do Windows.
- Corrigido o Círculo chegando ao jogo ao fechar o menu: o overlay desaparece imediatamente, mas o foco só é devolvido após a soltura física do botão.
- Corrigido o cursor visível atrás do menu: ele é ocultado continuamente, tem sua posição preservada e é restaurado ao fechar.
- Versão do aplicativo, instalador, interface, manifesto e VERSIONINFO atualizada para `1.0.0.21`.

## v1.0 Oficial - Build 20 - Base técnica profissional

- Adicionada a opção `Xbox Game Bar — Ligada/Desligada`, identificada por um `X` direto.
- Adicionada a opção `Ícone na barra de tarefas — Ligado/Desligado`, identificada pela bandeira do Windows e ligada por padrão.
- O ícone residente é recuperado automaticamente após a reinicialização do Explorer e aceita clique duplo para abrir o centro.
- A preferência anterior da Xbox Game Bar agora é preservada, reaplicada nas próximas execuções e restaurada na desinstalação.
- Incluído primeiro uso explícito para escolher o comportamento da Xbox Game Bar; a alteração deixou de ser silenciosa.
- Adicionada a ação `Encerrar DualCenter`, com confirmação, e comandos internos para fechamento e manutenção.
- Configurações corrompidas são preservadas com extensão `.corrupt`; as novas gravações usam substituição atômica com restauração do arquivo anterior em caso de falha.
- Instalador valida o destino, registra exatamente os arquivos próprios e não apaga recursivamente conteúdo desconhecido durante a remoção.
- Atualizações encerram a instância de forma normal e restringem o encerramento forçado ao PID da janela validada.
- Atalhos do Menu Iniciar e da Área de Trabalho passaram a ser criados pelas APIs nativas do Windows, sem PowerShell.
- O aplicativo instalado deixa de herdar a elevação administrativa do instalador.
- Versão centralizada em `internal/version/version.json`; aplicativo, instalador, interface, manifesto e VERSIONINFO usam `1.0.0.20`.
- Manifestos atualizados para Windows 10/11, long paths e DPI Per-Monitor V2.
- Build reproduzível com verificação de formatação, `go vet`, testes, SHA-256 e assinatura Authenticode opcional.
- Adicionados testes de persistência atômica, metadados, gerador de recursos e segurança do instalador, além de CI para Windows.
- Código de Game Bar, barra de tarefas, configurações, inicialização, manutenção e instalador foi separado em módulos menores.
- Nenhuma reformulação visual foi aplicada nesta etapa; o refinamento visual permanece separado.

## v1.1 Oficial - Build 19 - CPU em segundo plano otimizada

- Adicionado um cache direto para o último DualSense ativo, evitando consultas repetidas a vários mapas em cada relatório HID.
- A renovação interna durante a pressão longa do botão PS deixou de bloquear o estado em todo pacote, preservando a detecção de desconexão e o tempo de três segundos.
- Verificações de manutenção do Bluetooth foram espaçadas sem atrasar a primeira solicitação de relatório completo.
- O timer do menu foi reduzido de 20 para 10 verificações por segundo; PS, D-pad, X e O continuam processados imediatamente por Raw Input.
- Atualizações externas de áudio e estado dos jogos foram ajustadas para um e dois segundos, respectivamente, reduzindo consultas Win32 enquanto o menu está aberto.
- Mantidas a otimização de RAM do Build 18 e a correção do botão X do Build 17.

## v1.1 Oficial - Build 18 - Memória gráfica otimizada

- A superfície gráfica reutilizável do overlay agora é liberada assim que o menu, a bateria ou uma mensagem são ocultados.
- Artes do controle, cartões e molduras deixaram de manter uma segunda cópia em bitmap GDI; a composição usa diretamente os pixels pré-multiplicados já necessários para a janela layered.
- Removida a dependência de `msimg32.dll`, que era usada apenas pelo caminho gráfico duplicado.
- O pacote do projeto não inclui mais `installer/payload/DualCenter.exe`: essa cópia temporária já está incorporada no instalador final e é recriada automaticamente pelos scripts de build.
- O visual, a resposta do controle, os caches de renderização e as correções do Build 17 foram preservados.

## v1.1 Oficial - Build 17 - X isolado ao adicionar jogos

- Corrigido o X do DualSense sendo recebido pelo jogo ao confirmar o mosaico “Adicionar jogo”.
- O overlay permanece como janela ativa até o seletor de executável assumir o foco, eliminando a breve reativação do jogo durante a transição.
- O foco automático do menu e os comandos PS/D-pad/X/O ficam suspensos apenas enquanto o seletor está aberto.
- Ao fechar ou concluir o seletor, o menu retorna diretamente à grade da aba Jogos, com cursor e navegação por controle restaurados.

## v1.1 Oficial - Build 16 - Otimização segura de recursos

- Fontes, pincéis e canetas GDI agora são reutilizados entre redesenhos e liberados ao trocar de escala ou fechar o aplicativo.
- Mantidos exatamente os tamanhos, pesos, cores, bordas e posições usados no Build 15.
- O log passa a ser rotacionado ao chegar a 1 MiB, preservando apenas o arquivo atual e um backup `.old`.
- A incorporação do instalador foi restrita a `DualCenter.exe`; o placeholder `.gitkeep` foi removido.
- Scripts de build atualizados para formatar todos os arquivos Go da raiz.
- Adicionado teste automatizado para a rotação segura do log.

## v1.1 Oficial - Build 15 - Sombra do aviso sem recortes

- Adicionada uma margem transparente dedicada ao redor do cartão “Overlay indisponível”.
- A sombra agora termina completamente antes dos limites da janela, eliminando o recorte visível nos quatro cantos.
- Mantidos o tamanho, a posição visual, os textos e o restante do estilo aprovado no Build 14.

## v1.1 Oficial - Build 14 - Texto simplificado do aviso

- Removido o rótulo “MODO DE EXIBIÇÃO” do cartão de tela cheia exclusiva.
- Título alterado para “Overlay indisponível”.
- Mantida abaixo a instrução “Use Tela cheia sem borda ou Janela para abrir o menu.”.
- Título e instrução reposicionados para preservar o equilíbrio visual do cartão.

## v1.1 Oficial - Build 13 - Overlay de tela exclusiva profissional

- Criado um cartão exclusivo para o aviso de tela cheia, sem reutilizar o visual do overlay de bateria.
- Nova hierarquia com ícone de monitor, contexto, título curto e instrução objetiva.
- Fundo em vidro escuro com vinheta, reflexo superior, borda antialiasada e destaque azul discreto.
- Tipografia, contraste, espaçamento e dimensões ajustados proporcionalmente de 720p a 4K.
- Mantido o bloqueio seguro do menu em tela cheia exclusiva, sem roubar o foco do jogo.

## v1.1 Oficial - Build 12 - Borda do aviso de fullscreen

- Aumentada a margem horizontal entre o texto e as bordas do aviso de tela cheia exclusiva.
- Altura da cápsula ampliada para preservar o mesmo respiro nas bordas superior e inferior.
- Mantidos a centralização, a tipografia e o dimensionamento proporcional em 1080p e 4K.

## v1.1 Oficial - Build 11 - Otimização e limpeza

- Removida a espera artificial de 6 segundos na inicialização automática.
- Removidos estados de animação, modo sem ativação e chamadas Win32 que não eram mais usados.
- Timer do menu reduzido de 60 para 20 verificações por segundo, sem alterar a resposta do Raw Input.
- Superfície DIB do overlay agora é reutilizada enquanto o tamanho da janela não muda.
- Caches dependentes de escala são liberados ao trocar monitor/resolução ou quando a escala calculada muda.
- Varredura de processos reutiliza um único buffer em vez de alocar um buffer por processo.
- Gravação de configurações preserva o arquivo anterior se a substituição falhar.
- Instalador removeu pausas cosméticas, evita carregar executáveis inteiros na memória e mantém o arquivo anterior em falhas de substituição.
- Fechamento do instalador agora destrói a janela e libera fontes e pincéis corretamente.
- Scripts de build executam testes reais; adicionados testes de bateria, abas, jogos e identificação do controle.

## v1.1 Oficial - Aviso de fullscreen e tipografia

- As abas deixam de abrir quando um jogo está em tela cheia exclusiva.
- Exibido aviso compacto orientando o uso de tela cheia sem borda ou janela.
- Título em Segoe UI Variable Display Bold e texto auxiliar em Segoe UI Variable Text Medium.
- Largura calculada pelo texto para manter as bordas laterais rente à frase em 1080p e 4K.

## v1.0 Oficial - Limpeza final, fullscreen e instalador 4K

- Mantida a correção para jogos em Fullscreen exclusivo: o menu abre sem roubar foreground, evitando retorno para a Área de Trabalho.
- Removida a opção “Voltar ao jogo” da aba Jogos.
- Removido o overlay visual extra exibido após cancelar desligamento; a função de cancelar desligamento foi mantida.
- Removidos os assets e referências do overlay visual de cancelamento.
- Fechamento de jogo reforçado usando encerramento forçado quando o fechamento normal falhar.
- Instalador ajustado para DPI/4K: texto e controles nítidos em escala alta.
- Instalador abre centralizado na tela.
- Sem delay, sleep, timer extra ou debounce nos comandos.

- Removido o bloqueio residual da antiga janela de teste v38.
- Processos externos agora têm seus recursos liberados corretamente.
- Falhas nos comandos de energia passam a ser registradas no log.
- Scripts de build agora executam `go vet` e tratam caminhos de saída com segurança.
