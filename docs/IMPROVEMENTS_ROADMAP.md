# Plano de melhorias e correções — PicoFarm e Orca Slicer API

**Status:** planejamento, não implementação.
**Última consolidação:** 2026-11-14.
**Prioridade da marca/logo:** baixa, no fim do plano. A logo correta foi implantada; melhorias gráficas ficam como polimento opcional.

Este documento reúne problemas observados, propostas e critérios para decidir trabalhos futuros. Uma proposta não significa que o problema esteja reproduzido, que já exista uma solução aprovada ou que haja compromisso de prazo. Antes de cada ciclo: confirmar o estado atual em código/produção, definir escopo, risco, testes e critérios mensuráveis. Não incluir segredos nem detalhes do ambiente local aqui.

## Ordem sugerida

1. Proteção da restauração de banco (PicoFarm).
2. Limites efetivos para uploads e requisições (PicoFarm e slicer).
3. Orçamento de memória e controle de capacidade do slicer.
4. Otimização mensurável do frontend PicoFarm.
5. Robustez operacional do slicer: catálogo de perfis, cache e jobs.
6. Testes de interface e concorrência/polling.
7. Contratos OpenAPI e observabilidade.
8. Polimento da logo, por último.

Esta ordem prioriza risco de perda de dados e indisponibilidade antes de funcionalidades e acabamento visual.

---

## P0 — PicoFarm: restauração de backup segura

**Motivo/evidência:** auditoria `AUD-01` e `AUD-02` em [AUDIT_2026-11.md](AUDIT_2026-11.md): o caminho auditado fecha o banco, substitui o arquivo sem validar integridade/compatibilidade antes e não reabre o handle compartilhado. Confirmar novamente no código antes de iniciar.

**Decisões antes da implementação:**
- Restore offline com processo parado, ou modo de manutenção coordenado dentro do serviço?
- Quais formatos e versões de schema serão aceitos?
- Quanto espaço extra se reserva para arquivo temporário e backup pré-restore?
- O que deve acontecer com requisições e tarefas agendadas durante a operação?

**Proposta de escopo:** validar o backup numa cópia temporária (integridade SQLite e compatibilidade); criar cópia do estado atual e verificar seu sucesso; gravar a cópia restaurada em arquivo temporário no mesmo filesystem; sincronizar e trocar atomicamente; coordenar writers/handle e só então reabrir ou exigir procedimento offline claramente bloqueado. Erro em qualquer etapa deve preservar banco anterior e retornar estado operacional explícito. Não apagar o backup de segurança até verificar o banco restaurado.

**Aceitação/testes:** backups válidos, truncados, schema incompatível, falta de espaço/erro de I/O simulado, processo interrompido em cada fase, requisição concorrente e consulta após restore. Verificar que o banco anterior continua legível em falhas e que o modo de manutenção/reinício é inequívoco. Usar apenas diretórios/bancos temporários.

## P0 — Limites de upload e requisição

**Motivo/evidência:** `AUD-03` identifica falta de teto efetivo no upload de G-code; outros handlers multipart precisam ser verificados individualmente. O argumento de memória de `ParseMultipartForm` não é limite de bytes. A API do slicer também recebe modelos e perfis diretos; as suas proteções diferem por endpoint.

**Decisões:** limites separados por formato/endpoint, tamanho total da requisição, número máximo de partes/arquivos, compatibilidade com uploads legítimos existentes e eventual configuração por implantação.

**Proposta:** impor teto no corpo HTTP antes de parsear multipart, validar tamanho individual e agregado, limitar quantidade de partes, remover arquivos temporários em sucesso/erro e emitir `413` consistente. Não confiar somente em limite de proxy. Alinhar docs e mensagens frontend.

**Aceitação/testes:** fronteira abaixo/no/acima do limite; excesso agregado com partes individuais válidas; corpo chunked; JSON malformado; conexão interrompida; confirmação de que storage e temporários não deixam artefatos após rejeição. Sem testes que exauram disco real.

## P0 — Slicer: memória e capacidade

**Motivo/evidência:** `AUD-04` confirma que o PicoFarm copia o STL inteiro para memória antes de enviar slice/preview. No serviço slicer, examinar também buffering multipart, geração de thumbnail, cache e processos externos. Ainda não há medição de RSS nem carga para quantificar o risco.

**Proposta incremental:** primeiro definir modelo de recursos: máximo de bytes por entrada, memória disponível, timeout por fase e concorrência permitida. Medir heap/RSS com modelos sintéticos progressivos e chamadas concorrentes. Em seguida considerar streaming multipart para o backend PicoFarm e slicer, admission control com semáforo e fila limitada/rejeição explícita. Evitar “otimizar” sem perfil de memória comparável.

**Aceitação:** limites determinísticos; 413/429/503 documentados; cancelamento/timeout mata o processo filho quando possível; não deixar arquivos temporários nem jobs presos; carga sustentada e pico medidos com ambiente isolado e critérios definidos previamente.

## P1 — Otimização do frontend PicoFarm

**Por que:** Vite gera warning para o bundle principal acima de 600 kB. O build mais recente observado durante o trabalho marcou cerca de 945 kB minificados para o chunk principal, além de um chunk `three` de aproximadamente 514 kB. Esses números são pistas de tamanho, não medem por si só tempo real de carregamento. A auditoria não mediu latência ou Core Web Vitals.

**Plano de investigação e ganhos de baixo risco:**
1. Criar baseline repetível: tamanho bruto e gzip/Brotli por chunk, Lighthouse/Web Vitals em hardware/rede definidos, navegação até Dashboard, Projetos e detalhes; registrar cache frio e quente.
2. Aplicar lazy loading às rotas pouco usadas (ex.: 3D viewer, câmeras, timeline/relatórios) e dividir dependências pesadas para serem carregadas só na tela que as usa. Confirmar o grafo real antes de mover dependências.
3. Revisar o uso de `three`/visualizador STL: carregamento dinâmico, descarte de geometrias/materials/renderers e listeners ao desmontar; imagens/arquivos grandes não devem inicializar o visualizador sem necessidade.
4. Revisar bibliotecas de gráficos e ícones: imports tree-shakeable e carregamento de páginas/gráficos sob demanda; medir antes/depois.
5. Reduzir chamadas redundantes e waterfalls de dados usando os hooks TanStack Query existentes, com stale time e invalidação adequados; não trocar consistência por cache desatualizado.
6. Validar imagens, fontes, compressão HTTP e cache-control do servidor. Avaliar preload somente para recursos críticos medidos.

**Aceitação:** critérios acordados, por exemplo redução de pelo menos 25% no JS transferido na carga inicial (gzip/Brotli) sem piorar interação/erro; primeira rota não deve baixar chunks exclusivos de telas não visitadas. Fixar budgets por chunk no CI em vez de simplesmente aumentar `chunkSizeWarningLimit`. Comparar navegação, funcionalidades e acessibilidade.

## P1 — Slicer: catálogo built-in recursivo e descoberta de perfis

**Estado documentado:** [ROADMAP do slicer](../../slicer-api/docs/ROADMAP.md) informa que a rota rasa retorna 404 no layout vendor normal, embora resolução de perfis nomeados funcione. Confirmar repositório/branch antes de executar.

**Funcionalidade proposta:** listar perfis incluídos no pacote por categoria e fabricante, com paginação/filtros para não retornar milhares de entradas. Alternativa ainda mais simples: endpoint de busca por nome/vendor, sem expor árvore completa. Separar bem perfis do usuário e perfis built-in; não copiar automaticamente vendor JSON para armazenamento editável.

**Cuidados:** ignorar BOM/JSON inválido sem derrubar listagem; nomes duplicados e aliases; herança; symlinks/path traversal; custo de varrer 14k+ JSON no startup ou por request. Considerar índice criado uma vez no startup, limites de resultado, cache/invalidação por versão.

**Aceitação:** busca em categorias e fornecedores, comportamento determinístico para duplicatas, perfis não editáveis tratados como read-only, latência e memória medidas, testes contra árvore fixture pequena e uma validação no vendor tree real.

## P1 — Slicer: cache correto e ciclo de vida dos arquivos

**Motivo:** roadmap atual aponta possíveis lacunas em chave de cache, invalidação por perfis/versão e lifecycle de temporários. Primeiro auditar `ResultCache` e cenários em vez de presumir bugs.

**Proposta:** chave do cache incluir hash do modelo, perfis finais resolvidos, overrides, opções relevantes, versão do Orca e parâmetros que afetam saída; impedir que duas requisições concorrentes escrevam/limpem o mesmo caminho; definir TTL/limite em bytes e coleta segura.

**Aceitação:** testes de hits/misses quando muda cada entrada, concorrência sem corrupção, limites de disco e expiração, processo reiniciado sem servir resultado inválido. Medir taxa de hit e armazenamento antes/depois.

## P1 — Slicer: melhor experiência de fatiamento sem motor novo

As propostas abaixo são camadas em torno do CLI Orca, não mudanças no motor de fatiamento. Manter o Orca como fonte do resultado e expor as opções que já existam no CLI/API apenas após comprovar compatibilidade com a versão empacotada.

### A. Estimativa antes do slice

- Usar o endpoint preview existente como “pré-visualizar”, mostrando tempo estimado, material, camadas, warnings e dimensões antes de baixar G-code.
- Se o preview já existir mas a UI do PicoFarm não o usar, a melhoria é integração visual e estados, não duplicar endpoint.
- Avisar que estimativa não é medição garantida da impressora.
- Complexidade: baixa/média; exige teste do fluxo HTTP real e UI.

### B. Validar compatibilidade do perfil e mostrar warnings legíveis

- Apresentar quais perfis foram resolvidos, se houve fallback/alias, warning de ambiguidade, campos de override e categoria faltante.
- Rejeitar seleção ambígua em vez de escolher perfil arbitrário; o core já rejeita fallback de tokens ambíguo.
- Adicionar “ver JSON efetivo” sanitizado/readonly para suporte, sem modificar perfil armazenado.
- Complexidade: baixa/média; atenção para não revelar paths internos ou dados sensíveis.

### C. Perfil de fatiamento nomeado e reutilizável

- Salvar conjuntos de printer/process/filament + overrides em configuração da aplicação (PicoFarm), sem duplicar arquivos vendor.
- Permitir presets como “rápido”, “qualidade” ou “suportes” somente por parâmetros existentes comprovados no CLI.
- Versionar a configuração e exportar/importar; preview sempre mostra o conjunto efetivo.
- Complexidade: média; precisa definir propriedade/escopo por usuário (atualmente sem multiusuário/auth robustos).

### D. Cancelar trabalho assíncrono de forma efetiva

- Diferenciar “solicitação de cancelamento” do estado terminal; propagar contexto para processo filho e encerrar processo com timeout/cleanup.
- Confirmar que não há arquivo parcial servido como sucesso e documentar casos em que CLI não responde ao sinal.
- Complexidade: média; importante para controle de recursos, mas requer teste de subprocesso real.

### E. Melhorar saída G-code/3MF e relatórios

- Retornar metadados estruturados consistentes: versão do slicer, perfis efetivos/identificadores, tempo, camadas, material, warnings, tamanho, formato, hash.
- Expor link de download para resultado async sem manter dados somente em memória e sem perder resultado ao reiniciar.
- Confirmar formato e cabeçalhos; não afirmar impressão física validada.
- Complexidade: baixa para metadados síncronos; média para persistência durável de jobs.

### F. Comparação de presets / estimativas

- Permitir comparar preview de 2–3 combinações de perfis para o mesmo modelo (tempo/material/camadas/warnings), sem gerar múltiplos G-codes por padrão.
- Limitar paralelismo e custo por pedido; mostrar que estimates podem variar com firmware, aceleração e condições reais.
- Complexidade: média; dependerá de fila/limites antes de habilitar em produção.

**Sugestão de sequência no slicer:** B (warnings/identificação efetiva) → A (aproveitar preview existente) → E (metadados/download robusto) → catálogo de busca → C (presets salvos) → D/F depois de definir concorrência e cancelamento. Nenhuma proposta implica que já esteja implementada.

## P1 — UI: polling, erros e testes de navegador

**Estado:** AUD-05..08 têm correções anteriores em pedido, fila e projetos. Os testes existentes de pedido são isolados, e polling/projetos não têm testes interativos suficientes; confirmar estado nos commits atuais.

**Proposta:** proteger contra resposta fora de ordem/corrida entre polling e refresh; cancelar requisições ao desmontar/navegar; mostrar snapshot antigo como stale, sem chamar isso de estado atual. Adicionar testes de integração em navegador para cenário 500/timeout e sucesso vazio 200, cancelamento do prompt, expedição e erro de mutação.

**Aceitação:** timers controláveis, requests concorrentes resolvidos em ordem inversa, sem updates após unmount, alertas acessíveis e estados distintos para vazio real/erro. Não repetir ação ambígua sem consultar estado do servidor.

## P2 — Backup: evitar colisão de nomes

**Motivo:** AUD-09: precisão de nome em segundos pode fazer duas solicitações concorrerem pelo mesmo destino. Confirmar implementação atual.

**Proposta:** nome único com timestamp de maior precisão e/ou identificador aleatório, criação exclusiva e retry limitado; relógio injetável em testes.

**Aceitação:** duas chamadas simultâneas produzem arquivos distintos e íntegros; falha de escrita preserva arquivos prévios.

## P2 — OpenAPI e contratos

**Motivo:** divergências entre especificação manual e handlers estão documentadas no slicer. PicoFarm também exige sincronizar Go e TypeScript.

**Proposta:** gerar/verificar OpenAPI ou teste de contrato a partir de fixtures; conferir códigos HTTP, multipart, parâmetros, respostas de jobs, erros e auth. Corrigir docs somente depois de confirmar comportamento real.

**Aceitação:** CI falha em incompatibilidade intencional; exemplos curl executáveis contra servidor de teste; não documentar endpoint inexistente.

## P2 — Observabilidade operacional

- PicoFarm: duração/falha de uploads e restore, estado de jobs e tamanho de storage, sem logar tokens ou payload sensível.
- Slicer: duração por fase, fila/admissão, exit status/timeouts, bytes de entrada/saída, uso de cache, limpeza de temporários.
- Definir métricas, retenção e alertas sem alta cardinalidade (não usar nome de arquivo, URL de perfil ou ID não limitado como label).
- Complexidade baixa/média conforme métricas; alertas dependem de limites acordados.

## P3 — Marca/logo (menor prioridade)

A logo corrigida usa PNG quadrado 1024×1024, servida no cabeçalho em tamanho pequeno com `object-contain`; o navegador reamostra a imagem, o que pode suavizar bordas. Não degradar o original nem converter automaticamente sem revisão.

Opções somente depois dos itens de maior prioridade:
1. criar SVG vetorial manual/revisado a partir da forma simples, preservar transparência/cor e comparar visualmente em dark/light e alta densidade;
2. exportar PNGs em tamanhos específicos (favicon, 2x cabeçalho) com downsampling de boa qualidade e conferir nitidez;
3. manter o PNG atual caso a melhoria não seja mensurável.

Aceitação: sem halo/crop, fundo/transparência corretos, aparência consistente em 16–64 CSS px e não aumentar carga inicial de modo significativo.

---

## Fora do escopo deste documento

- Não há autorização para começar qualquer melhoria descrita aqui.
- Não tratar o baseline OrcaSlicer 2.4.0 e saída 2.5.0 como equivalentes; não houve validação em impressão física.
- Não assumir suporte ARM64 real: imagem atual do slicer é amd64-only.
- Não alterar API tokens ou adotar segurança/autenticação nova sem plano específico.
- Não usar dados privados, backups de produção ou impressoras reais em testes iniciais.

## Como iniciar cada item

Para abrir um ciclo de implementação, registrar: hipótese reproduzível; caminhos atuais; risco/severidade; decisão de produto necessária; limites numéricos de carga/bytes/latência; testes de unidade/integrados; plano de rollback; documentação a atualizar. Trabalhar um item pequeno por commit e executar os gates locais e CI. Push/deploy é etapa separada e exige autorização explícita; verificar revisão da imagem e comportamento funcional após qualquer redeploy.
