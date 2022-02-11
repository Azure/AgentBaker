for s in scenarios/*/; do
    echo $s
    scenario=$(echo $s | cut -d'/' -f2)
    echo $scenario
done