define retrycmd_if_failure
    retries=$(1); wait_sleep=$(2); cmd=$(3); target=$$(basename $$(echo $(3))); \
    echo "Running $$cmd with $$retries retries"; \
    for i in $$(seq 1 $$retries); do \
        $$cmd && break || \
        if [ $$i -eq $$retries ]; then \
            echo "$$target failed $$i times"; \
            exit 1; \
        else \
            sleep $$wait_sleep; \
        fi \
    done
endef