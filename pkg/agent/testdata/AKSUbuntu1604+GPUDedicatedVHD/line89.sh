#!/bin/bash

#enable MIG mode
#nvidia-smi -mig 1
nvidia-smi mig -cgi 9,9
nvidia-smi mig -cci 