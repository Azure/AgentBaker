#!/bin/bash
# shellcheck disable=SC2329

# ShellSpec tests for parseAutologinSessions helper function

Describe 'parseAutologinSessions helper function'
  # Extract only the function definition using sed - clean approach!
  BeforeAll "eval \"\$(sed -n '/^parseAutologinSessions()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  Describe 'parseAutologinSessions'
    It 'returns empty when no sessions exist'
      loginctl() {
        if [ "$1" = "list-sessions" ]; then
          echo ""
        fi
      }
      When call parseAutologinSessions
      The output should equal ""
    End

    It 'returns empty when only SSH sessions exist (Remote=yes)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "1 1000 user seat0"
            ;;
          "show-session")
            cat <<EOF
Remote=yes
Service=sshd
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal ""
    End

    It 'returns session ID when console autologin session found (Remote=no + Service=login)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "42 1000 core seat0"
            ;;
          "show-session")
            cat <<EOF
Remote=no
Service=login
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal "42"
    End

    It 'returns multiple session IDs when multiple autologin sessions exist'
      loginctl() {
        case "$1" in
          "list-sessions")
            cat <<EOF
1 1000 user seat0
2 1001 core seat1
3 1002 admin seat2
EOF
            ;;
          "show-session")
            cat <<EOF
Remote=no
Service=login
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The line 1 of output should equal "1"
      The line 2 of output should equal "2"
      The line 3 of output should equal "3"
    End

    It 'returns only autologin sessions when mixed with SSH sessions'
      loginctl() {
        case "$1" in
          "list-sessions")
            cat <<EOF
1 1000 user seat0
2 1001 core pts/0
3 1002 admin seat1
EOF
            ;;
          "show-session")
            case "$2" in
              "1")
                cat <<EOF
Remote=no
Service=login
Type=tty
EOF
                ;;
              "2")
                cat <<EOF
Remote=yes
Service=sshd
Type=tty
EOF
                ;;
              "3")
                cat <<EOF
Remote=no
Service=login
Type=tty
EOF
                ;;
            esac
            ;;
        esac
      }
      When call parseAutologinSessions
      The line 1 of output should equal "1"
      The line 2 of output should equal "3"
    End

    It 'detects format changes when both Remote and Service fields are missing'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "99 1000 user seat0"
            ;;
          "show-session")
            # Missing both Remote= and Service= fields - format changed!
            cat <<EOF
Type=tty
State=active
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal "PARSE_ERROR:99"
      The error should include "Cannot parse loginctl output"
      The error should include "format may have changed"
    End

    It 'handles sessions with only Remote field present (partial data)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "98 1000 user seat0"
            ;;
          "show-session")
            cat <<EOF
Remote=yes
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal ""
    End

    It 'handles sessions with only Service field present (partial data)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "97 1000 user seat0"
            ;;
          "show-session")
            cat <<EOF
Service=login
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal ""
    End


    It 'detects when loginctl show-session fails or returns empty'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "99 1000 user seat0"
            ;;
          "show-session")
            # Simulate failure - returns empty output
            return 1
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal "PARSE_ERROR:99"
      The error should include "Command may have failed"
    End
  End
End
