# Plano de melhorias e correções — PicoFarm e Orca Slicer API

**Status:** planejamento com incrementos locais implementados; não implica deploy.
**Última consolidação:** 2026-11-20.

## Decisões posteriores registradas

- [Orçamento e custeio](QUOTING.md): decisões de produto aprovadas para FDM, múltiplos projetos/peças/componentes, preço dinâmico por último campo editado, proposta inicial em PDF e condições comerciais simples. Isso é escopo planejado, não funcionalidade entregue.
- **Nova prioridade aprovada para investigação:** duplicação e arranjo de instâncias do mesmo STL na mesa, espaçamento, reorientação e visualização de camadas/trajetórias do G-code. A [análise de viabilidade](SLICING_FEASIBILITY.md) separa o preview de G-code já armazenado, a multiplicação pelo CLI Orca e o preview anterior ao fatiamento, com gates mensuráveis. Nenhum renderer ou contrato novo foi implementado ou selecionado.

## Funcionalidade futura — mesas com múltiplos STLs

**Registro:** 2026-11-22. **Status:** ideia registrada para investigação futura; sem autorização de implementação, prioridade ou prazo definidos. As alternativas abaixo não são decisões de produto aprovadas.

**Objetivo:** combinar STLs diferentes, com quantidade própria por modelo, em uma única mesa e um único G-code, buscando aproveitar a mesa e reduzir intervenções. Economia de tempo não é garantida: deverá ser medida considerando deslocamentos, suportes e risco de falha compartilhado.

**Compatibilidade obrigatória:** preservar a biblioteca e o fluxo atual de fatiamento de um STL com suas cópias. Não associar artificialmente um G-code misto a apenas um dos STLs nem alterar silenciosamente o contrato individual existente.

**Organização a avaliar:**
- Selecionar vários STLs na biblioteca e acionar “Criar mesa”, sem mover ou duplicar os originais.
- Uma área de mesas salvas para combinações reutilizáveis; avaliar sua necessidade antes de adicionar navegação.
- Uma mesa referencia modelos e quantidades; cada G-code gerado conserva um retrato imutável da composição, identidade/conteúdo dos arquivos e parâmetros efetivamente utilizados. Editar a mesa não altera o significado de resultados anteriores.
- Distinguir quantidade por modelo de número de repetições da mesa. Distinguir peças independentes de partes alinhadas de um mesmo objeto.
- Manter a interface enxuta: ações e estado visíveis, explicações em ajuda sob demanda, erros e riscos relevantes apresentados no contexto.

**Hipótese de primeira versão, ainda não aprovada:** um material e um conjunto compartilhado de perfis, arranjo automático e uma única mesa. Posicionamento manual, multimaterial, parâmetros por objeto, impressão sequencial, otimização automática e vínculos com produtos/pedidos ficam fora dessa hipótese inicial.

**Estudos e critérios antes de implementar:**
- Comprovar no Orca CLI empacotado o carregamento de modelos distintos com quantidades diferentes e verificar a identidade e quantidade de cada modelo no resultado; contar apenas o total de objetos não basta.
- Rejeitar peças omitidas, redução de quantidade ou divisão em mesas não solicitada. Não fundir os modelos em uma malha única como atalho sem avaliar perda de identidade e semântica.
- Definir contrato explícito de composição, persistência/migração, comportamento ao excluir/substituir um STL e rastreabilidade das versões de G-code.
- Incluir todos os modelos, quantidades, transformações e parâmetros nas chaves de cache/deduplicação e na invalidação da prévia; manter paridade entre preview e slice.
- Medir tempo, memória, disco e payloads; separar capacidade de fatiamento do limite do preview 3D, sem aumentar tetos apenas para acomodar mesas maiores.
- Validar regressões do fluxo individual e testar composição válida, quantidades distintas, falta de espaço, entrada inválida, timeout, falha e limpeza de temporários, sem comandos às impressoras.

**Perguntas abertas:** uso ocasional, kits recorrentes ou produção de produtos/pedidos diferentes? Onde descobrir mesas e seus G-codes sem confundir a biblioteca atual? Como registrar produção parcial e distribuir custos caso haja integração produtiva? Essas decisões deverão ser retomadas com o usuário antes de fechar o escopo.

## Incremento do fatiador implementado nesta rodada

- Contrato preview/slice alinhado, booleanos explícitos e multipart temporário em disco no PicoFarm. Não há benchmark de RSS ou disco; o orçamento global de recursos abaixo continua pendente.
- Comparação de 2–4 G-codes existentes do mesmo STL, preview desatualizado identificado e paginação local de 50 grupos. Comparação de novos cenários não foi implementada.
- Viewer com dimensões/eixos originais, unidade mm assumida, vistas rápidas e render sob demanda. Preview limitado a 32 MiB/500 mil triângulos; parsing ainda síncrono.
- Editor Validate → Save, conflito por hash e snapshots imutáveis. Preflight parcial, não certificação Orca ou de segurança física. Ver limites em API/security.
- Fila local PicoFarm com uma execução e oito esperando, sem replay após restart. Ver [SLICER_JOBS.md](SLICER_JOBS.md).
- No repositório do slicer: suporte do preview preservado e cache isolado por entradas/perfis/executável; testes com CLI sintético, sem validação Orca real nesta rodada.

### Seleção aprovada ainda pendente

Preservação de encomendas antigas; comparação A/B de novos fatiamentos; página Simulações; margem de material; compras por demanda; rastreabilidade e custo final de bobinas; organização em massa (primeiro tags/vínculos); simulador comercial; estoque pronto; remessas parciais; visões salvas; controle de notificações. Não são funcionalidades entregues.

### Delimitação da seleção

Ficam para futuro explícito: painel de pendências, contagem boa/refugo/faltante, roteiro de bancada, perfis rápidos de teste e aproveitamento de sobras. Repetir encomenda e licenças comerciais continuam condicionais. Novos overrides no modal não foram aprovados.

Descartados nesta seleção: substituição de material, reutilização de receita G-code, painel independente de resolução de perfis, superfície de mesa, relatório de upload, experimentos, briefing, aprovação pública, revisão comercial, CRM, sinal/saldo, financeiro por pedido, personalização por item, pós-venda e busca global. A validação dentro do editor permanece no escopo. Tokens não foram alterados; logo continua na menor prioridade.

As propostas abaixo são backlog de investigação e não autorização automática para implementar itens descartados ou futuros.
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

**Motivo/evidência histórica:** `AUD-04` identificou cópia integral do STL em memória no PicoFarm. Nesta rodada o multipart de slice/preview passou a temporário em disco; isso não limita tamanho nem comprova o consumo global. No serviço slicer, examinar também buffering multipart, geração de thumbnail, cache e processos externos. Ainda não há medição de RSS nem carga para quantificar o risco.

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
