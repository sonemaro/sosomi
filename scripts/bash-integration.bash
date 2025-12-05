# Sosomi Bash Integration
# Add this to your .bashrc:
# source /path/to/sosomi/scripts/bash-integration.bash

# Sosomi function with readline integration
sosomi-prompt() {
    local prompt="$READLINE_LINE"
    
    if [[ -z "$prompt" ]]; then
        echo ""
        read -r -p "🐚 sosomi> " prompt
        echo ""
    else
        READLINE_LINE=""
        READLINE_POINT=0
        echo ""
    fi
    
    if [[ -n "$prompt" ]]; then
        sosomi "$prompt"
        echo ""
    fi
}

# Bind to Ctrl+G
bind -x '"\C-g": sosomi-prompt'

# Add sosomi to PATH if installed locally
if [[ -d "$HOME/.local/bin" ]] && [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    export PATH="$HOME/.local/bin:$PATH"
fi

# Completions
_sosomi_complete() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    
    # Main commands
    local commands="chat config history undo models"
    
    # Options
    local opts="-a --auto -d --dry-run -e --explain -s --silent -m --model -p --provider --profile --force --config -h --help -v --version"
    
    case "$prev" in
        -p|--provider)
            COMPREPLY=( $(compgen -W "openai ollama lmstudio llamacpp" -- "$cur") )
            return 0
            ;;
        --profile)
            COMPREPLY=( $(compgen -W "strict moderate permissive" -- "$cur") )
            return 0
            ;;
        --config)
            COMPREPLY=( $(compgen -f -- "$cur") )
            return 0
            ;;
        -m|--model)
            # Could potentially query available models here
            return 0
            ;;
    esac
    
    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "$opts" -- "$cur") )
    elif [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
    fi
    
    return 0
}

complete -F _sosomi_complete sosomi

# Aliases
alias s='sosomi'
alias sq='sosomi --silent'
alias sd='sosomi --dry-run'
alias sa='sosomi --auto'

echo "🐚 Sosomi shell integration loaded. Press Ctrl+G to invoke."
