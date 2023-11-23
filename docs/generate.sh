#!/bin/bash

TEMPLATES="doc=./templates/doc.gotxt,example=./templates/example.gotxt,file=./templates/file.gotxt,func=./templates/func.gotxt,import=./templates/import.gotxt,index=./templates/index.gotxt,list=./templates/list.gotxt,package=./templates/package.gotxt,text=./templates/text.gotxt,type=./templates/type.gotxt,value=./templates/value.gotxt"

gomarkdoc \
  --template-file="$TEMPLATES" \
  -e -o \
  ./source/includes/generated/_client.md ../client/.

gomarkdoc \
  --template-file="$TEMPLATES" \
  -e -o \
  ./source/includes/generated/_backend.md ../backend/.
