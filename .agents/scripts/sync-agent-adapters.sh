#!/usr/bin/env bash
# Делает скилы из .agents/skills и правила из .agents/guidance видимыми для всех агентов.
#
# Кто что умеет:
#
#   агент         скилы                                  правила
#   ------------- -------------------------------------- ---------------------------
#   Claude Code   .claude/skills/<name>        (в git)    AGENTS.md → .agents/guidance
#   opencode      .opencode/skills/<name>      (в git)    AGENTS.md → .agents/guidance
#   Cursor        ~/.cursor/skills/<name>      (локально) .cursor/rules/<name>.mdc (в git)
#   Codex         ~/.codex/skills/<name>       (локально) AGENTS.md → .agents/guidance
#
# Проектные скилы поддерживают только Claude Code и opencode — для них адаптеры лежат в репозитории
# и работают у всех. Cursor и Codex читают скилы лишь из своих глобальных каталогов, поэтому туда
# ставится symlink с абсолютным путем: он привязан к машине и в git не попадает, так что каждый
# разработчик запускает синхронизацию у себя.
#
# Использование:
#   .agents/scripts/sync-agent-adapters.sh          # создать/обновить/удалить адаптеры
#   .agents/scripts/sync-agent-adapters.sh --check  # только проверить актуальность (для CI)

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
MODE="${1:-sync}"

# Проектные каталоги скилов: путь относительно корня репозитория.
PROJECT_SKILL_DIRS=(
  ".claude/skills"
  ".opencode/skills"
)

# Глобальные каталоги скилов: абсолютный путь, можно переопределить переменной окружения.
GLOBAL_SKILL_DIRS=(
  "${CODEX_SKILLS_DIR:-$HOME/.codex/skills}"
  "${CURSOR_SKILLS_DIR:-$HOME/.cursor/skills}"
)

case "$MODE" in
  sync|--check) ;;
  *)
    echo "Неизвестный аргумент: $MODE (ожидается --check или без аргументов)" >&2
    exit 2
    ;;
esac

errors=0
changes=0

note_change() {
  changes=$((changes + 1))
  printf '%s\n' "$1"
}

short_path() {
  local path="$1"
  case "$path" in
    "$ROOT"/*) printf '%s' "${path#"$ROOT"/}" ;;
    "$HOME"/*) printf '~/%s' "${path#"$HOME"/}" ;;
    *) printf '%s' "$path" ;;
  esac
}

ensure_link() {
  local link="$1"
  local target="$2"
  local actual=""

  if [[ -L "$link" ]]; then
    actual="$(readlink "$link")"
    [[ "$actual" == "$target" ]] && return 0
  elif [[ -e "$link" ]]; then
    echo "Нельзя обновить adapter: $(short_path "$link") существует и не является symlink" >&2
    errors=$((errors + 1))
    return 0
  fi

  if [[ "$MODE" == "--check" ]]; then
    echo "Устарел adapter: $(short_path "$link") -> $target" >&2
    errors=$((errors + 1))
    return 0
  fi

  [[ ! -L "$link" ]] || rm "$link"
  ln -s "$target" "$link"
  note_change "adapter: $(short_path "$link") -> $target"
}

remove_stale_link() {
  local link="$1"

  if [[ "$MODE" == "--check" ]]; then
    echo "Лишний adapter: $(short_path "$link")" >&2
    errors=$((errors + 1))
    return 0
  fi

  rm "$link"
  note_change "adapter удален: $(short_path "$link")"
}

# Скил — это каталог .agents/skills/<name> с файлом SKILL.md внутри.
list_skills() {
  find "$ROOT/.agents/skills" -mindepth 2 -maxdepth 2 -type f -name 'SKILL.md' -print0 |
    LC_ALL=C sort -z
}

skill_exists() {
  [[ -f "$ROOT/.agents/skills/$1/SKILL.md" ]]
}

# Удаляет symlink'и, которые ведут в .agents/skills этого репозитория, но чей скил уже удален.
# Чужие ссылки (глобальные скилы пользователя, скилы других проектов) не трогаем.
drop_stale_skill_links() {
  local dir="$1" prefix="$2" link actual name

  [[ -d "$dir" ]] || return 0

  while IFS= read -r -d '' link; do
    actual="$(readlink "$link")"
    case "$actual" in
      "$prefix"*)
        name="${actual#"$prefix"}"
        skill_exists "$name" || remove_stale_link "$link"
        ;;
    esac
  done < <(find "$dir" -maxdepth 1 -type l -print0 | LC_ALL=C sort -z)
}

# Claude Code, opencode: проектные скилы, относительный путь, адаптеры коммитятся.
sync_project_skills() {
  local rel_dir="$1" dir skill_file name

  dir="$ROOT/$rel_dir"
  mkdir -p "$dir"

  while IFS= read -r -d '' skill_file; do
    name="$(basename "$(dirname "$skill_file")")"
    ensure_link "$dir/$name" "../../.agents/skills/$name"
  done < <(list_skills)

  drop_stale_skill_links "$dir" "../../.agents/skills/"
}

# Cursor, Codex: только глобальный каталог, поэтому абсолютный путь.
sync_global_skills() {
  local dir="$1" skill_file name

  if [[ ! -d "$dir" ]]; then
    if [[ "$MODE" == "--check" ]]; then
      echo "Пропускаю $(short_path "$dir"): каталога нет"
      return 0
    fi
    mkdir -p "$dir"
  fi

  while IFS= read -r -d '' skill_file; do
    name="$(basename "$(dirname "$skill_file")")"
    ensure_link "$dir/$name" "$ROOT/.agents/skills/$name"
  done < <(list_skills)

  drop_stale_skill_links "$dir" "$ROOT/.agents/skills/"
}

# Cursor: правила из .agents/guidance подключаются как .cursor/rules/<name>.mdc.
# Формат совпадает — у guidance-файлов уже есть frontmatter с description и alwaysApply.
sync_cursor_rules() {
  local dir="$ROOT/.cursor/rules" source name link actual source_from_link

  mkdir -p "$dir"

  while IFS= read -r -d '' source; do
    name="$(basename "$source" .md)"
    ensure_link "$dir/$name.mdc" "../../.agents/guidance/$name.md"
  done < <(find "$ROOT/.agents/guidance" -maxdepth 1 -type f -name '*.md' -print0 | LC_ALL=C sort -z)

  while IFS= read -r -d '' link; do
    actual="$(readlink "$link")"
    case "$actual" in
      ../../.agents/guidance/*)
        source_from_link="${actual#../../.agents/guidance/}"
        if [[ ! -f "$ROOT/.agents/guidance/$source_from_link" ||
              "$link" != "$dir/$(basename "$source_from_link" .md).mdc" ]]; then
          remove_stale_link "$link"
        fi
        ;;
    esac
  done < <(find "$dir" -maxdepth 1 -type l -print0 | LC_ALL=C sort -z)
}

for skill_dir in "${PROJECT_SKILL_DIRS[@]}"; do
  sync_project_skills "$skill_dir"
done

for skill_dir in "${GLOBAL_SKILL_DIRS[@]}"; do
  sync_global_skills "$skill_dir"
done

sync_cursor_rules

if (( errors > 0 )); then
  [[ "$MODE" != "--check" ]] || echo "Запусти: make agent-adapters" >&2
  exit 1
fi

if [[ "$MODE" == "--check" ]]; then
  echo "AI adapter symlinks актуальны."
elif [[ "$changes" -eq 0 ]]; then
  echo "AI adapter symlinks уже актуальны."
fi
