package completion

import (
	"fmt"
	"strings"

	"github.com/jmagar/nugs-cli/internal/ui"
)

// Command handles the `nugs completion <shell>` command.
func Command(args []string) error {
	if len(args) < 2 {
		ui.PrintInfo("Usage: nugs completion <shell>")
		fmt.Println("Supported shells: bash, zsh, fish, powershell")
		fmt.Println("")
		fmt.Println("Installation examples:")
		fmt.Println("  Bash:       nugs completion bash > /etc/bash_completion.d/nugs")
		fmt.Println("  Zsh:        nugs completion zsh > ~/.zsh/completion/_nugs")
		fmt.Println("  oh-my-zsh:  nugs completion zsh > ~/.oh-my-zsh/custom/completions/_nugs")
		fmt.Println("  Fish:       nugs completion fish > ~/.config/fish/completions/nugs.fish")
		fmt.Println("  PowerShell: nugs completion powershell > $PROFILE")
		fmt.Println("")
		fmt.Println("Note: For oh-my-zsh, add this to .zshrc BEFORE sourcing oh-my-zsh.sh:")
		fmt.Println("      fpath=($ZSH/custom/completions $fpath)")
		return nil
	}

	shell := strings.ToLower(args[1])
	switch shell {
	case "bash":
		fmt.Print(BashCompletion)
	case "zsh":
		fmt.Print(ZshCompletion)
	case "fish":
		fmt.Print(FishCompletion)
	case "powershell", "pwsh":
		fmt.Print(PowershellCompletion)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", shell)
	}

	return nil
}

// BashCompletion is the bash completion script.
const BashCompletion = `# Nugs CLI bash completion script
# Installation: nugs completion bash > /etc/bash_completion.d/nugs
# Or: nugs completion bash > ~/.bash_completion.d/nugs

_nugs_completion() {
    local cur prev words cword
    _init_completion || return

    # Top-level commands
    local commands="list catalog watch status cancel help completion"

    # Flags
    local flags="-f -F -o --force-video --skip-videos --skip-chapters --json --help"

    # Handle flag completions
    case "$prev" in
        -f)
            COMPREPLY=($(compgen -W "1 2 3 4 5" -- "$cur"))
            return
            ;;
        -F)
            COMPREPLY=($(compgen -W "1 2 3 4 5" -- "$cur"))
            return
            ;;
        -o)
            COMPREPLY=($(compgen -d -- "$cur"))
            return
            ;;
        --json)
            COMPREPLY=($(compgen -W "minimal standard extended raw" -- "$cur"))
            return
            ;;
    esac

    # Command-specific completions
    if [[ $cword -eq 1 ]]; then
        # First argument: commands or numeric show ID
        COMPREPLY=($(compgen -W "$commands $flags" -- "$cur"))
        return
    fi

    case "${words[1]}" in
        list)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "artists" -- "$cur"))
            elif [[ $cword -eq 3 && "${words[2]}" == "artists" ]]; then
                COMPREPLY=($(compgen -W "shows" -- "$cur"))
            elif [[ $cword -eq 3 ]]; then
                # After artist ID
                COMPREPLY=($(compgen -W "shows latest" -- "$cur"))
            fi
            ;;
        catalog)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "update cache stats latest list gaps coverage config" -- "$cur"))
            elif [[ "${words[2]}" == "config" && $cword -eq 3 ]]; then
                COMPREPLY=($(compgen -W "enable disable set" -- "$cur"))
            elif [[ "${words[2]}" == "gaps" && $cword -ge 4 ]]; then
                COMPREPLY=($(compgen -W "fill --ids-only" -- "$cur"))
            fi
            ;;
        watch)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "add remove list check enable disable" -- "$cur"))
            fi
            ;;
        completion)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "$cur"))
            fi
            ;;
    esac
}

complete -F _nugs_completion nugs
`

// ZshCompletion is the zsh completion script.
const ZshCompletion = `#compdef nugs
# Nugs CLI zsh completion script
# Installation: nugs completion zsh > ~/.zsh/completion/_nugs
# Then add to ~/.zshrc: fpath=(~/.zsh/completion $fpath)

_nugs() {
    local -a commands catalog_cmds config_cmds watch_cmds
    commands=(
        'list:List artists or shows'
        'catalog:Catalog management commands'
        'watch:Artist watch management'
        'status:Show runtime status'
        'cancel:Cancel running crawl'
        'help:Display help'
        'completion:Generate shell completion scripts'
    )

    catalog_cmds=(
        'update:Update catalog cache'
        'cache:Show cache status'
        'stats:Show catalog statistics'
        'latest:Show latest additions'
        'list:List shows for artist'
        'gaps:Find missing shows'
        'coverage:Show collection coverage'
        'config:Configure auto-refresh'
    )

    config_cmds=(
        'enable:Enable auto-refresh'
        'disable:Disable auto-refresh'
        'set:Configure auto-refresh settings'
    )

    watch_cmds=(
        'add:Add artist to watch list'
        'remove:Remove artist from watch list'
        'list:Show watched artists'
        'check:Update catalog and download new shows'
        'enable:Enable systemd watch timer'
        'disable:Disable systemd watch timer'
    )

    _arguments -C \
        '-f[Track download format (1-5)]:format:(1 2 3 4 5)' \
        '-F[Video download format (1-5)]:format:(1 2 3 4 5)' \
        '-o[Output path]:path:_directories' \
        '--force-video[Force video download]' \
        '--skip-videos[Skip videos in artist URLs]' \
        '--skip-chapters[Skip video chapters]' \
        '--json[JSON output level]:level:(minimal standard extended raw)' \
        '--help[Show help]' \
        '1: :->cmds' \
        '*:: :->args'

    case $state in
        cmds)
            _describe -t commands 'nugs commands' commands
            ;;
        args)
            case $words[1] in
                list)
                    if [[ $CURRENT -eq 2 ]]; then
                        _values 'list subcommands' 'artists[List all artists]'
                    elif [[ $words[2] == "artists" && $CURRENT -eq 3 ]]; then
                        _values 'filter options' 'shows[Filter by show count]'
                    elif [[ $CURRENT -eq 3 ]]; then
                        _values 'artist subcommands' 'shows[Filter by venue]' 'latest[Latest N shows]'
                    fi
                    ;;
                catalog)
                    if [[ $CURRENT -eq 2 ]]; then
                        _describe -t catalog_cmds 'catalog commands' catalog_cmds
                    elif [[ $words[2] == "config" && $CURRENT -eq 3 ]]; then
                        _describe -t config_cmds 'config commands' config_cmds
                    elif [[ $words[2] == "gaps" ]]; then
                        _values 'gap options' 'fill[Download missing shows]' '--ids-only[Show IDs only]'
                    fi
                    ;;
                watch)
                    if [[ $CURRENT -eq 2 ]]; then
                        _describe -t watch_cmds 'watch commands' watch_cmds
                    fi
                    ;;
                completion)
                    if [[ $CURRENT -eq 2 ]]; then
                        _values 'shells' 'bash' 'zsh' 'fish' 'powershell'
                    fi
                    ;;
            esac
            ;;
    esac
}

_nugs "$@"
`

// FishCompletion is the fish completion script.
const FishCompletion = `# Nugs CLI fish completion script
# Installation: nugs completion fish > ~/.config/fish/completions/nugs.fish

# Disable file completion by default
complete -c nugs -f

# Top-level commands
complete -c nugs -n "__fish_use_subcommand" -a "list" -d "List artists or shows"
complete -c nugs -n "__fish_use_subcommand" -a "catalog" -d "Catalog management"
complete -c nugs -n "__fish_use_subcommand" -a "watch" -d "Artist watch management"
complete -c nugs -n "__fish_use_subcommand" -a "status" -d "Show runtime status"
complete -c nugs -n "__fish_use_subcommand" -a "cancel" -d "Cancel running crawl"
complete -c nugs -n "__fish_use_subcommand" -a "help" -d "Display help"
complete -c nugs -n "__fish_use_subcommand" -a "completion" -d "Generate shell completions"

# Global flags
complete -c nugs -s f -d "Track download format (1-5)" -a "1 2 3 4 5"
complete -c nugs -s F -d "Video download format (1-5)" -a "1 2 3 4 5"
complete -c nugs -s o -d "Output path" -r
complete -c nugs -l force-video -d "Force video download"
complete -c nugs -l skip-videos -d "Skip videos in artist URLs"
complete -c nugs -l skip-chapters -d "Skip video chapters"
complete -c nugs -l json -d "JSON output level" -a "minimal standard extended raw"
complete -c nugs -l help -d "Show help"

# list command
complete -c nugs -n "__fish_seen_subcommand_from list" -n "test (count (commandline -opc)) -eq 2" -a "artists" -d "List all artists"
complete -c nugs -n "__fish_seen_subcommand_from list; and __fish_seen_argument -s artists" -a "shows" -d "Filter by show count"
complete -c nugs -n "__fish_seen_subcommand_from list; and test (count (commandline -opc)) -eq 3" -a "shows" -d "Filter shows by venue"
complete -c nugs -n "__fish_seen_subcommand_from list; and test (count (commandline -opc)) -eq 3" -a "latest" -d "Show latest N shows"

# catalog command
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "update" -d "Update catalog cache"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "cache" -d "Show cache status"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "stats" -d "Show catalog statistics"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "latest" -d "Show latest additions"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "list" -d "List shows for artist"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "gaps" -d "Find missing shows"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "coverage" -d "Show collection coverage"
complete -c nugs -n "__fish_seen_subcommand_from catalog" -n "test (count (commandline -opc)) -eq 2" -a "config" -d "Configure auto-refresh"

# catalog config subcommands
complete -c nugs -n "__fish_seen_subcommand_from catalog; and __fish_seen_argument -s config" -a "enable" -d "Enable auto-refresh"
complete -c nugs -n "__fish_seen_subcommand_from catalog; and __fish_seen_argument -s config" -a "disable" -d "Disable auto-refresh"
complete -c nugs -n "__fish_seen_subcommand_from catalog; and __fish_seen_argument -s config" -a "set" -d "Configure auto-refresh settings"

# catalog gaps options
complete -c nugs -n "__fish_seen_subcommand_from catalog; and __fish_seen_argument -s gaps" -a "fill" -d "Download missing shows"
complete -c nugs -n "__fish_seen_subcommand_from catalog; and __fish_seen_argument -s gaps" -l ids-only -d "Show IDs only"

# watch command
complete -c nugs -n "__fish_seen_subcommand_from watch" -n "test (count (commandline -opc)) -eq 2" -a "add" -d "Add artist to watch list"
complete -c nugs -n "__fish_seen_subcommand_from watch" -n "test (count (commandline -opc)) -eq 2" -a "remove" -d "Remove artist from watch list"
complete -c nugs -n "__fish_seen_subcommand_from watch" -n "test (count (commandline -opc)) -eq 2" -a "list" -d "Show watched artists"
complete -c nugs -n "__fish_seen_subcommand_from watch" -n "test (count (commandline -opc)) -eq 2" -a "check" -d "Update catalog and download new shows"
complete -c nugs -n "__fish_seen_subcommand_from watch" -n "test (count (commandline -opc)) -eq 2" -a "enable" -d "Enable systemd watch timer"
complete -c nugs -n "__fish_seen_subcommand_from watch" -n "test (count (commandline -opc)) -eq 2" -a "disable" -d "Disable systemd watch timer"

# completion command
complete -c nugs -n "__fish_seen_subcommand_from completion" -n "test (count (commandline -opc)) -eq 2" -a "bash" -d "Bash completion"
complete -c nugs -n "__fish_seen_subcommand_from completion" -n "test (count (commandline -opc)) -eq 2" -a "zsh" -d "Zsh completion"
complete -c nugs -n "__fish_seen_subcommand_from completion" -n "test (count (commandline -opc)) -eq 2" -a "fish" -d "Fish completion"
complete -c nugs -n "__fish_seen_subcommand_from completion" -n "test (count (commandline -opc)) -eq 2" -a "powershell" -d "PowerShell completion"
`

// PowershellCompletion is the PowerShell completion script.
const PowershellCompletion = `# Nugs CLI PowerShell completion script
# Installation: nugs completion powershell >> $PROFILE
# Or create a separate file and dot-source it in $PROFILE

Register-ArgumentCompleter -Native -CommandName nugs -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @{
        'list' = 'List artists or shows'
        'catalog' = 'Catalog management commands'
        'watch' = 'Artist watch management'
        'status' = 'Show runtime status'
        'cancel' = 'Cancel running crawl'
        'help' = 'Display help'
        'completion' = 'Generate shell completion scripts'
    }

    $catalogCommands = @{
        'update' = 'Update catalog cache'
        'cache' = 'Show cache status'
        'stats' = 'Show catalog statistics'
        'latest' = 'Show latest additions'
        'list' = 'List shows for artist'
        'gaps' = 'Find missing shows'
        'coverage' = 'Show collection coverage'
        'config' = 'Configure auto-refresh'
    }

    $configCommands = @{
        'enable' = 'Enable auto-refresh'
        'disable' = 'Disable auto-refresh'
        'set' = 'Configure auto-refresh settings'
    }

    $watchCommands = @{
        'add' = 'Add artist to watch list'
        'remove' = 'Remove artist from watch list'
        'list' = 'Show watched artists'
        'check' = 'Update catalog and download new shows'
        'enable' = 'Enable systemd watch timer'
        'disable' = 'Disable systemd watch timer'
    }

    $shells = @('bash', 'zsh', 'fish', 'powershell')
    $jsonLevels = @('minimal', 'standard', 'extended', 'raw')
    $trackFormats = @('1', '2', '3', '4', '5')

    # Parse command line
    $tokens = $commandAst.ToString() -split '\s+'
    $position = $tokens.Count - 1

    # Handle flag completions
    if ($wordToComplete -match '^-') {
        $flags = @(
            [System.Management.Automation.CompletionResult]::new('-f', '-f', 'ParameterName', 'Track download format (1-5)')
            [System.Management.Automation.CompletionResult]::new('-F', '-F', 'ParameterName', 'Video download format (1-5)')
            [System.Management.Automation.CompletionResult]::new('-o', '-o', 'ParameterName', 'Output path')
            [System.Management.Automation.CompletionResult]::new('--force-video', '--force-video', 'ParameterName', 'Force video download')
            [System.Management.Automation.CompletionResult]::new('--skip-videos', '--skip-videos', 'ParameterName', 'Skip videos')
            [System.Management.Automation.CompletionResult]::new('--skip-chapters', '--skip-chapters', 'ParameterName', 'Skip chapters')
            [System.Management.Automation.CompletionResult]::new('--json', '--json', 'ParameterName', 'JSON output level')
            [System.Management.Automation.CompletionResult]::new('--help', '--help', 'ParameterName', 'Show help')
        )
        return $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    }

    # Previous token for context-aware completions
    $prev = if ($position -gt 0) { $tokens[$position - 1] } else { '' }

    # Handle flag value completions
    switch ($prev) {
        '-f' {
            return $trackFormats | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', "Format $_")
            }
        }
        '-F' {
            return $trackFormats | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', "Format $_")
            }
        }
        '--json' {
            return $jsonLevels | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', "JSON level: $_")
            }
        }
    }

    # First argument: top-level commands
    if ($position -eq 1) {
        return $commands.GetEnumerator() | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_.Key, $_.Key, 'ParameterValue', $_.Value)
        } | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    }

    # Command-specific completions
    $cmd = $tokens[1]
    switch ($cmd) {
        'list' {
            if ($position -eq 2) {
                return [System.Management.Automation.CompletionResult]::new('artists', 'artists', 'ParameterValue', 'List all artists')
            }
            elseif ($position -eq 3 -and $tokens[2] -eq 'artists') {
                return [System.Management.Automation.CompletionResult]::new('shows', 'shows', 'ParameterValue', 'Filter by show count')
            }
            elseif ($position -eq 3) {
                return @(
                    [System.Management.Automation.CompletionResult]::new('shows', 'shows', 'ParameterValue', 'Filter by venue')
                    [System.Management.Automation.CompletionResult]::new('latest', 'latest', 'ParameterValue', 'Latest N shows')
                )
            }
        }
        'catalog' {
            if ($position -eq 2) {
                return $catalogCommands.GetEnumerator() | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_.Key, $_.Key, 'ParameterValue', $_.Value)
                } | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            }
            elseif ($position -eq 3 -and $tokens[2] -eq 'config') {
                return $configCommands.GetEnumerator() | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_.Key, $_.Key, 'ParameterValue', $_.Value)
                } | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            }
            elseif ($tokens[2] -eq 'gaps') {
                return @(
                    [System.Management.Automation.CompletionResult]::new('fill', 'fill', 'ParameterValue', 'Download missing shows')
                    [System.Management.Automation.CompletionResult]::new('--ids-only', '--ids-only', 'ParameterValue', 'Show IDs only')
                )
            }
        }
        'watch' {
            if ($position -eq 2) {
                return $watchCommands.GetEnumerator() | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_.Key, $_.Key, 'ParameterValue', $_.Value)
                } | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            }
        }
        'completion' {
            if ($position -eq 2) {
                return $shells | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', "$_ shell completion")
                } | Where-Object { $_ -like "$wordToComplete*" }
            }
        }
    }
}
`
