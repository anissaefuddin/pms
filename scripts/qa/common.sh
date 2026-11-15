# Variabel bersama. QA_INSTANCE=a|b memberi proyek compose, port dan volume terpisah (uji isolasi).
: "${QA_INSTANCE:=a}"
case "$QA_INSTANCE" in a) QA_PG_PORT=${QA_PG_PORT:-55514}; QA_APP_PORT=${QA_APP_PORT:-18091};; b) QA_PG_PORT=${QA_PG_PORT:-55524}; QA_APP_PORT=${QA_APP_PORT:-18092};; *) echo "QA_INSTANCE harus a atau b" >&2; exit 2;; esac
export QA_INSTANCE QA_PG_PORT QA_APP_PORT
: "${QA_IMAGE:=pms:qa-b3d-86c33d1}"; export QA_IMAGE
DC="docker compose -f $(dirname "$0")/compose.yml"
BASEURL="http://localhost:$QA_APP_PORT"
