# Tutorial completo: testando o HarnessForge

Este guia testa o fluxo principal em um repositório descartável. Comece pelas etapas locais, que não exigem chave de IA nem enviam conteúdo a provedores externos. Depois, se desejar, teste busca com IA e plugins de forma explícita.

## 1. Instale e confirme a versão

No macOS ou Linux, instale a release publicada:

```sh
curl -fsSLO https://github.com/joicepassos/harness-forge/releases/download/v1.0.2/install.sh
sh install.sh --version 1.0.2 --install-dir "$HOME/.local/bin"
```

No Windows PowerShell:

```powershell
Invoke-WebRequest https://github.com/joicepassos/harness-forge/releases/download/v1.0.2/install.ps1 -OutFile .\install.ps1
.\install.ps1 -Version 1.0.2 -InstallDir "$env:USERPROFILE\bin"
```

Adicione o diretório escolhido ao `PATH` ou abra um novo terminal. Confirme:

```powershell
$env:Path = "$env:USERPROFILE\bin;$env:Path"
harnessforge version
harnessforge --language pt-BR --help
```

Para adicionar o diretório ao `PATH` do seu usuário permanentemente, execute o bloco abaixo uma vez e abra um novo PowerShell:

```powershell
$InstallDir = "$env:USERPROFILE\bin"
$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($UserPath -split ';' | Where-Object { $_ }) -notcontains $InstallDir) {
  [Environment]::SetEnvironmentVariable('Path', "$UserPath;$InstallDir", 'User')
}
```

> Para conferir o script e as verificações de checksum antes de executá-lo, veja [Instalação](INSTALLATION.md).

## 2. Crie um projeto de teste

Não faça o primeiro teste no seu repositório de produção. Crie uma pasta temporária com alguns documentos:

```powershell
$Project = Join-Path $HOME 'harnessforge-tutorial'
New-Item -ItemType Directory -Force $Project | Out-Null
Set-Location $Project
git init
New-Item -ItemType Directory -Force docs | Out-Null
Set-Content README.md '# Loja de exemplo'
Set-Content docs\architecture.md 'A API usa autenticação por token e registra pedidos no banco de dados.'
git add .
git commit -m 'chore: add tutorial fixture'
```

## 3. Entenda o repositório sem alterá-lo

`analyze` é local e somente leitura. O resultado mostra linguagens, arquivos e, com `--git`, metadados já presentes no clone:

```powershell
harnessforge analyze --git .
harnessforge analyze --git --format json . | Set-Content analysis.json
```

Abra `analysis.json` e confirme se a descrição corresponde ao pequeno projeto criado.

## 4. Crie e revise o harness

Inicialize o arquivo que guarda decisões revisáveis para agentes:

```powershell
harnessforge init
Get-Content .harness\harness.yaml
harnessforge validate --repository .
```

Abra `.harness\harness.yaml` no editor. Adicione ou revise uma regra aprovada, por exemplo:

```yaml
rules:
  - id: run-tests-before-review
    description: Execute a suíte de testes antes de pedir revisão.
    origin: human
    status: approved
```

Valide novamente. Se o arquivo já tiver uma seção `rules`, acrescente o item nela em vez de criar uma segunda seção.

```powershell
harnessforge validate --repository .
```

## 5. Gere instruções de agente

Com o YAML válido, gere os arquivos que os agentes leem:

```powershell
harnessforge generate codex --repository .
harnessforge generate claude --repository .
Get-Content AGENTS.md
Get-Content CLAUDE.md
```

Confira se apenas regras com `status: approved` aparecem. Os comandos não substituem arquivos manuais que não pertencem ao HarnessForge.

## 6. Verifique saúde e alterações

Estas verificações não modificam o projeto:

```powershell
harnessforge doctor .harness\harness.yaml --repository .
harnessforge drift .harness\harness.yaml --repository .
```

Mude a descrição de uma regra ou um arquivo usado como evidência e execute `drift` novamente. O resultado é um aviso para revisão humana, não uma alteração automática.

## 7. Indexe e pesquise documentos localmente

O índice é salvo em `.harness\index.json`. A busca abaixo não chama um modelo de texto:

```powershell
harnessforge index .
harnessforge search . 'Como a API autentica pedidos?' --k 5
```

Você deve ver um trecho de `docs\architecture.md` com caminho e linhas de origem.

## 8. Inspecione o contexto antes de usar IA

Veja exatamente quais arquivos poderiam ser selecionados para uma pergunta. Esta etapa também é local:

```powershell
harnessforge context explain . 'Como a API autentica pedidos?'
```

Confirme os caminhos incluídos, excluídos e o limite de contexto. Arquivos sensíveis, binários, links simbólicos e caminhos ignorados aparecem como excluídos ou não têm seu conteúdo enviado.

## 9. Opcional: resposta com IA e citações

Só prossiga se você tiver autorização para enviar o conteúdo do projeto ao provedor escolhido. Defina a chave apenas no terminal atual; não a salve em YAML, `.env`, argumentos ou arquivos do projeto.

```powershell
$env:OPENAI_API_KEY = 'defina-a-no-seu-terminal'
harnessforge rag . 'Como a API autentica pedidos?' --k 5
```

O resultado deve citar o documento recuperado. Se não houver evidência relevante, o comando retorna `insufficient_evidence` em vez de fabricar uma resposta. Remova a variável quando terminar:

```powershell
Remove-Item Env:OPENAI_API_KEY
```

Leia a [política de segurança](../SECURITY.md) antes de testar outros provedores.

## 10. Opcional: execute o plugin de exemplo

Plugins são programas locais. Só autorize um plugin que você tenha revisado:

```powershell
Set-Location C:\caminho\para\harness-forge
harnessforge plugin discover .\examples\plugins
harnessforge plugin run .\examples\plugins\word-count word-count '{"text":"dois termos"}' --authorize
```

O exemplo deve retornar uma contagem de palavras. `--authorize` é obrigatório em toda execução e não é isolamento de segurança: um plugin autorizado tem as permissões do usuário que o executa.

## Resultado esperado

Ao final, você terá confirmado:

1. Instalação de uma release e identificação da versão.
2. Análise local de um repositório.
3. Criação, validação e geração de instruções a partir do Harness IR.
4. Diagnóstico e detecção de drift sem mudanças automáticas.
5. Indexação, busca e inspeção de contexto locais.
6. Opcionalmente, resposta com citações e execução explícita de plugin.

Para fluxos mais avançados, consulte o [guia de uso](USAGE.md) e o [mapa de comandos](USAGE.md#command-map).
