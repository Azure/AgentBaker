#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 86/364: 'configure_bashrc_exec_tmux'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

bash_config_file="/etc/profile"



if ! grep -xq '  if ! ancestor_procs | grep -q "tmux"; then exec tmux; fi' ${bash_config_file}; then

    cat >> ${bash_config_file} <<'EOF'


# There are cases were "$PS1" is not a good indicator of interacivity,  use $- instead
case  "$-" in *i*) 
  # Mariner sets hidepid, so unless we are root we may not see our parent process.
  #  Make a best effort not to start nested tmux sessions
  ancestor_procs() {
    pid=$$
    name=$(ps -o comm= -p $pid)
    echo "$name"
    while [ "$pid" -gt 1 ] && pid=$(ps -o ppid= -p $pid); do
      ps -o comm= -p $pid
    done
  }
  if ! ancestor_procs | grep -q "tmux"; then exec tmux; fi
  unset ancestor_procs
  ;;
esac

EOF
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
