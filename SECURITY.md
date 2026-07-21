# Segurança

## Relato de vulnerabilidades

Não publique detalhes exploráveis antes de uma correção. Envie ao mantenedor uma descrição do impacto, versão afetada, passos mínimos para reprodução e, quando possível, uma sugestão de correção.

## Garantias do instalador

- O destino é normalizado e validado antes da gravação.
- A raiz da unidade, a pasta do Windows, links de diretório e pastas com conteúdo desconhecido são recusados.
- Um marcador registra os arquivos pertencentes à instalação.
- A desinstalação remove somente os arquivos registrados e preserva conteúdo adicional.
- O encerramento forçado de uma atualização é restrito ao PID da janela do DualCenter.
- A preferência original da Xbox Game Bar é restaurada na remoção.

## Assinatura

Builds públicos devem ser assinados com Authenticode e timestamp SHA-256. O certificado e sua senha não devem ser adicionados ao repositório. O script `build.ps1` aceita a impressão digital e a URL de timestamp por variáveis de ambiente.
