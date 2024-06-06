#!/usr/local/lib/node_modules/bats/bin/bats

mock() {
  local name=$1
  local arr="${name}__calls"

  eval "${arr}=()"
  eval "$name() {
    ${arr}+=(\"${name} \$*\")
  }"
}

unmock() {
   local name=$1
   unset -f $name
}

assert_called() {
  local name=$1
  local arr="${name}__calls"

  assert_array_contains "${arr}" "${*}"
}

assert_array_contains() {
  local -n arr=$1
  local val=$2

  local found=false
  for i in "${arr[@]}"
  do
    if [[ "$i" == "${val}" ]] ; then
      found=true
    fi
  done

  if [[ "$found" == "false" ]] ; then
      echo "${arr[*]}"
      [[ "$found" == "true" ]]
  fi
}
