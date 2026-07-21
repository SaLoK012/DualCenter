# Privacidade

O DualCenter não cria conta, não envia telemetria e não possui serviço de análise. A leitura do controle, as preferências, os jogos cadastrados e os logs permanecem no computador.

## Dados armazenados

Em `%LOCALAPPDATA%\DualCenter` o aplicativo mantém:

- `settings.json`: opções do overlay, ordem das abas, caminhos dos jogos cadastrados, preferência da Xbox Game Bar e o valor necessário para restaurá-la.
- `DualCenter.log` e `DualCenter.log.old`: eventos técnicos e erros locais. O log é rotacionado em aproximadamente 1 MiB.

O instalador oferece a escolha de remover esses arquivos durante a desinstalação.

## Acesso ao sistema

O DualCenter lê relatórios HID de controles Sony compatíveis, consulta dispositivos de áudio, verifica os executáveis cadastrados para indicar se um jogo está aberto e altera somente preferências solicitadas pelo usuário no Registro do Windows.
