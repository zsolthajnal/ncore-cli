package main

import (
	"fmt"
	"os"
)

func runCompletion(args []string) {
	shell := ""
	if len(args) > 0 {
		shell = args[0]
	}
	if shell == "" {
		fmt.Fprintln(os.Stderr, "usage: ncore-cli completion <bash|zsh|fish>")
		os.Exit(1)
	}
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown shell %q (supported: bash, zsh, fish)\n", shell)
		os.Exit(1)
	}
}

const bashCompletion = `# ncore-cli bash completion
# Add to ~/.bashrc:
#   source <(ncore-cli completion bash)
# Or install system-wide:
#   ncore-cli completion bash | sudo tee /etc/bash_completion.d/ncore-cli

_ncore_cli_completion() {
    local cur prev words cword
    _init_completion 2>/dev/null || {
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    }

    local commands="search s download d scan ls watch w install i uninstall completion"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
        return
    fi

    local watch_flags="--media-dir --torrent-dir --state --interval --download-cmd --require --exclude --once"

    case "${words[1]}" in
    watch|w|install|i)
        case "$prev" in
        --media-dir|--torrent-dir|--state)
            COMPREPLY=( $(compgen -d -- "$cur") )
            ;;
        --interval)
            COMPREPLY=( $(compgen -W "15m 30m 1h 2h 6h 12h 24h" -- "$cur") )
            ;;
        --require|--exclude)
            COMPREPLY=( $(compgen -W "1080p 720p 480p 2160p 4K UHD WEB-DL BluRay x265 HEVC HUN" -- "$cur") )
            ;;
        *)
            COMPREPLY=( $(compgen -W "$watch_flags" -- "$cur") )
            ;;
        esac
        ;;
    scan|ls)
        COMPREPLY=( $(compgen -d -- "$cur") )
        ;;
    download|d)
        ;;
    completion)
        COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
        ;;
    esac
}

complete -F _ncore_cli_completion ncore-cli
`

const zshCompletion = `#compdef ncore-cli
# ncore-cli zsh completion
# Add to ~/.zshrc:
#   source <(ncore-cli completion zsh)
# Or install to your fpath:
#   ncore-cli completion zsh > "${fpath[1]}/_ncore-cli"

_ncore_cli() {
    local state

    _arguments -C \
        '1: :->command' \
        '*: :->args'

    case $state in
    command)
        local commands=(
            'search:Search torrents on ncore.pro'
            's:Search torrents on ncore.pro'
            'download:Download a torrent file by ID'
            'd:Download a torrent file by ID'
            'scan:List series and detected episodes in a media directory'
            'ls:List series and detected episodes in a media directory'
            'watch:Watch for new episodes and download automatically'
            'w:Watch for new episodes and download automatically'
            'install:Install ncore-cli as a systemd service'
            'i:Install ncore-cli as a systemd service'
            'uninstall:Stop and remove the ncore-cli systemd service'
            'completion:Output shell completion script'
        )
        _describe 'command' commands
        ;;
    args)
        case $words[2] in
        watch|w|install|i)
            _arguments \
                '--media-dir[Media directory to watch]:dir:_files -/' \
                '--torrent-dir[Staging dir for .torrent files]:dir:_files -/' \
                '--state[Path to state JSON file]:file:_files' \
                '--interval[Check interval]:duration:(15m 30m 1h 2h 6h 12h 24h)' \
                '--download-cmd[Download command template]:cmd:' \
                '--require[Required terms, comma-separated]:terms:(1080p 720p HUN WEB-DL BluRay)' \
                '--exclude[Excluded terms, comma-separated]:terms:(2160p 4K UHD HEVC)' \
                '--once[Run one cycle then exit]'
            ;;
        scan|ls)
            _arguments '*:media dir:_files -/'
            ;;
        download|d)
            _arguments \
                '1:torrent ID:' \
                '2:output dir:_files -/'
            ;;
        completion)
            local shells=(bash zsh fish)
            _describe 'shell' shells
            ;;
        esac
        ;;
    esac
}

_ncore_cli
`

const fishCompletion = `# ncore-cli fish completion
# Install:
#   ncore-cli completion fish > ~/.config/fish/completions/ncore-cli.fish

set -l commands search s download d scan ls watch w install i uninstall completion

# Disable file completion by default
complete -c ncore-cli -f

# Commands
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'search s'     -d 'Search torrents'
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'download d'   -d 'Download torrent by ID'
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'scan ls'      -d 'List series in media dir'
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'watch w'      -d 'Watch for new episodes'
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'install i'    -d 'Install systemd service'
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'uninstall'    -d 'Remove systemd service'
complete -c ncore-cli -n "not __fish_seen_subcommand_from $commands" -a 'completion'   -d 'Output shell completion script'

# watch / install flags
for sub in watch w install i
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l media-dir    -d 'Media directory'          -r -a '(__fish_complete_directories)'
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l torrent-dir  -d 'Torrent staging dir'      -r -a '(__fish_complete_directories)'
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l state        -d 'State JSON file path'     -r
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l interval     -d 'Check interval'           -r -a '15m 30m 1h 2h 6h 12h 24h'
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l download-cmd -d 'Download command template' -r
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l require      -d 'Required terms'           -r -a '1080p 720p HUN WEB-DL BluRay'
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l exclude      -d 'Excluded terms'           -r -a '2160p 4K UHD HEVC'
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -l once         -d 'Run one cycle then exit'
end

# scan / ls: directory arg
for sub in scan ls
    complete -c ncore-cli -n "__fish_seen_subcommand_from $sub" -a '(__fish_complete_directories)'
end

# completion: shell arg
complete -c ncore-cli -n "__fish_seen_subcommand_from completion" -a 'bash zsh fish'
`
