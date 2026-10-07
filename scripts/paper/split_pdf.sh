#!/bin/sh
# Splits the full manuscript PDF into the main text (front matter, body, references) and the appendices, from one LaTeX build so
# that every "Appendix X" and table number in the main text matches the supplement. The split is at the page that carries the
# "Appendix" heading (see main.tex). Usage: split_pdf.sh [input.pdf] [main_out.pdf] [supplement_out.pdf]
set -eu
IN=${1:-docs/paper.pdf}; MAIN=${2:-docs/paper_main.pdf}; SUPP=${3:-docs/paper_supplement.pdf}
N=$(pdfinfo "$IN" | awk '/^Pages:/ {print $2}')
SPLIT=0; p=1
while [ $p -le $N ]; do
  if pdftotext -f $p -l $p "$IN" - | awk 'NF {print; exit}' | grep -qx "Appendix"; then SPLIT=$p; break; fi
  p=$((p+1))
done
[ $SPLIT -gt 1 ] || { echo "no Appendix heading page found in $IN" >&2; exit 1; }
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
pdfseparate "$IN" "$T/p-%d.pdf" 2>/dev/null
set --; i=1; while [ $i -lt $SPLIT ]; do set -- "$@" "$T/p-$i.pdf"; i=$((i+1)); done
pdfunite "$@" "$MAIN" 2>/dev/null
set --; i=$SPLIT; while [ $i -le $N ]; do set -- "$@" "$T/p-$i.pdf"; i=$((i+1)); done
# the first loop's arguments were consumed by set --, so $@ holds only the appendix pages here
pdfunite "$@" "$SUPP" 2>/dev/null
echo "main: pages 1-$((SPLIT-1)) -> $MAIN; supplement: pages $SPLIT-$N -> $SUPP"
