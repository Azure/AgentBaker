#!/bin/bash -eu

compile_go_fuzzer github.com/Azure/agentbaker/fuzz/csecmd Fuzz fuzz_csecmd
compile_go_fuzzer github.com/Azure/agentbaker/fuzz/customdata Fuzz fuzz_customdata
