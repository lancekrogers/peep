set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

import 'just/build.just'
import 'just/dev.just'
import 'just/test.just'

default:
    @just --list
