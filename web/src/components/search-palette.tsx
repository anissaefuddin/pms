import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SearchResultList, SearchScopes } from "@/components/search-results";
import { getItem, itemKeys } from "@/lib/items";
import { effectiveScope, useSearchResults, type SearchContext, type SearchScope } from "@/lib/search";
import { useDebounced } from "@/lib/space-queries";

export function SearchPalette({ spaceId, containerId, hideTrigger = false }: SearchContext & { hideTrigger?: boolean }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [choice, setChoice] = useState<SearchScope>("everywhere");
  const input = useRef<HTMLInputElement>(null);
  const contextItem = useQuery({
    queryKey: itemKeys.detail(containerId ?? ""), queryFn: () => getItem(containerId!),
    enabled: open && !!containerId, staleTime: Infinity, retry: false,
  }).data?.item;
  const location = contextItem?.locations[0];
  const context: SearchContext = {
    spaceId: spaceId ?? location?.space_id,
    containerId: contextItem?.type === "task" ? location?.parent_item_id ?? undefined : contextItem?.id,
  };
  const scope = effectiveScope(choice, context);
  const debounced = useDebounced(query.trim(), 200);
  const search = useSearchResults(debounced, scope, context, 5, open);

  function changeOpen(next: boolean) {
    setOpen(next);
    if (next) {
      setQuery("");
      setChoice("everywhere");
    }
  }
  useEffect(() => {
    function openPalette() {
      changeOpen(true);
      input.current?.focus();
    }
    function shortcut(event: globalThis.KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        openPalette();
      }
    }
    window.addEventListener("keydown", shortcut);
    window.addEventListener("pms:open-search", openPalette);
    return () => {
      window.removeEventListener("keydown", shortcut);
      window.removeEventListener("pms:open-search", openPalette);
    };
  }, []);

  function moveFocus(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const results = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("[data-search-result]"));
    if (!results.length) return;
    const current = results.indexOf(event.target as HTMLElement);
    if (current === -1 && event.target !== input.current) return;
    event.preventDefault();
    const next = event.key === "ArrowDown"
      ? (current + 1) % results.length
      : (current <= 0 ? results.length - 1 : current - 1);
    results[next]?.focus();
  }

  const pending = query.trim() !== debounced || search.isPending;
  return <Dialog open={open} onOpenChange={changeOpen}>
    {!hideTrigger && <DialogTrigger render={<Button type="button" variant="ghost" size="sm" aria-label={t("search.title")}
      aria-keyshortcuts="Control+K Meta+K" />}>
      <Search className="size-4" />{t("search.title")}
    </DialogTrigger>}
    <DialogContent initialFocus={input} onKeyDown={moveFocus} className="sm:max-w-xl">
      <DialogTitle>{t("search.title")}</DialogTitle>
      <Input ref={input} type="search" value={query} maxLength={200} onChange={(event) => setQuery(event.target.value)}
        aria-label={t("search.keyword")} placeholder={t("search.placeholder")} />
      <SearchScopes context={context} scope={scope} onChange={setChoice} />
      <div className="max-h-[45dvh] overflow-y-auto">
        <SearchResultList data={search.data} query={query.trim()} pending={pending} error={search.isError}
          retry={() => void search.refetch()} close={() => changeOpen(false)} />
      </div>
      {query.trim().length >= 2 && <Link to="/search" search={{ q: query.trim(), scope, ...context }}
        className="rounded-md px-3 py-1 font-medium text-primary hover:underline focus-visible:outline-2 focus-visible:outline-ring"
        onClick={() => changeOpen(false)} data-search-result>{t("search.showAll")}</Link>}
    </DialogContent>
  </Dialog>;
}
