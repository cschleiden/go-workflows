#!/bin/bash

docker run --rm --name slate -p 4567:4567 \
  -v "$(pwd)/source/logo.png":/srv/slate/source/images/logo.png \
  -v "$(pwd)/source/index.html.md":/srv/slate/source/index.html.md \
  -v "$(pwd)/source/includes":/srv/slate/source/includes \
  -v "$(pwd)/build":/srv/slate/build slatedocs/slate build

cp ./images/* ./build/images/
