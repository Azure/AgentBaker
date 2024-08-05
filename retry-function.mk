define retrycmd_if_failure
    retries=$(1); wait_sleep=$(2); cmd=$(3); target=$$(basename $$(echo $(3))); \
    echo -e "\n==========================================================="; \
    echo -e "Running $$cmd with $$retries retries"; \
    for i in $$(seq 1 $$retries); do \
        $$cmd >> output-$${target%.*}.txt 2>&1 && break || \
        if [ $$i -eq $$retries ]; then \
            echo "$$target failed $$i times"; \
            cat output-$${target%.*}.txt; \
            exit 1; \
        else \
            sleep $$wait_sleep; \
            echo -e "\n\n\nNext Attempt:\n\n\n" >> output-$${target%.*}.txt 2>&1; \
        fi \
    done; \
    cat output-$${target%.*}.txt
endef