# Sosomi ZSH Integration
# Add this to your .zshrc:
# source /path/to/sosomi/scripts/zsh-integration.zsh

# Define the sosomi widget
function sosomi-widget() {
    local prompt
    
    # If there's text on the line, use it as the prompt
    if [[ -n "$BUFFER" ]]; then
        prompt="$BUFFER"
        BUFFER=""
        zle reset-prompt
    else
        # Otherwise, ask for input
        echo ""
        read -r "prompt?🐚 sosomi> "
        echo ""
    fi
    
    if [[ -n "$prompt" ]]; then
        # Run sosomi with the prompt
        local result
        result=$(sosomi "$prompt" 2>&1)
        
        # Show the result
        echo "$result"
        
        # Add a blank line before the prompt
        echo ""
    fi
    
    zle reset-prompt
}

# Create the widget
zle -N sosomi-widget

# Bind to Ctrl+G (since Ctrl+L is usually clear screen)
bindkey '^G' sosomi-widget

# Alternative binding for Ctrl+S (may need to disable flow control first)
# To disable flow control: stty -ixon
# bindkey '^S' sosomi-widget

# Add sosomi to PATH if installed locally
if [[ -d "$HOME/.local/bin" ]] && [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    export PATH="$HOME/.local/bin:$PATH"
fi

# Completions
_sosomi() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
        '-a[Auto-execute safe commands]' \
        '--auto[Auto-execute safe commands]' \
        '-d[Dry-run mode]' \
        '--dry-run[Dry-run mode]' \
        '-e[Explain only]' \
        '--explain[Explain only]' \
        '-s[Silent mode]' \
        '--silent[Silent mode]' \
        '-m[Model to use]:model:->models' \
        '--model=[Model to use]:model:->models' \
        '-p[Provider]:provider:(openai ollama lmstudio llamacpp)' \
        '--provider=[Provider]:provider:(openai ollama lmstudio llamacpp)' \
        '--profile=[Safety profile]:profile:(strict moderate permissive)' \
        '--force[Override safety blocks]' \
        '--config=[Config file path]:file:_files' \
        '-h[Show help]' \
        '--help[Show help]' \
        '-v[Show version]' \
        '--version[Show version]' \
        '*:prompt:' \
        '1:command:(chat config history undo models)'
}

compdef _sosomi sosomi

# Aliases
alias s='sosomi'
alias sq='sosomi --silent'
alias sd='sosomi --dry-run'
alias sa='sosomi --auto'

echo "🐚 Sosomi shell integration loaded. Press Ctrl+G to invoke."
