#!/bin/bash

cat term/pid | xargs -I{} kill {}
rm term/*
