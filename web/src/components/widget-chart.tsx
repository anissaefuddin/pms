import { useEffect, useRef } from "react";
import { BarChart, LineChart, PieChart } from "echarts/charts";
import { AriaComponent, GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import { SVGRenderer } from "echarts/renderers";
import { palette, type Grid } from "@/lib/charts";

echarts.use([BarChart, LineChart, PieChart, GridComponent, LegendComponent, TooltipComponent, AriaComponent, SVGRenderer]);

export type ChartProps = {
  type: "pie" | "column" | "bar" | "line";
  grid: Grid;
  /** Pie slices (one series). */
  slices: { name: string; value: number }[];
  showLegend: boolean;
  showLabels: boolean;
  pieStyle: "classic" | "donut" | "ring";
  metric: string;
  summary: string;
};

const radius = { classic: ["0%", "68%"], donut: ["42%", "68%"], ring: ["60%", "68%"] } as const;

/** One ECharts SVG chart. Colors come from the --cat tokens and are re-read when the theme class changes. */
export default function WidgetChart(props: ChartProps) {
  const host = useRef<HTMLDivElement>(null);
  const chart = useRef<echarts.ECharts | undefined>(undefined);
  useEffect(() => {
    if (!host.current) return;
    const instance = echarts.init(host.current, undefined, { renderer: "svg" });
    chart.current = instance;
    const resize = new ResizeObserver(() => instance.resize());
    resize.observe(host.current);
    return () => { resize.disconnect(); instance.dispose(); };
  }, []);
  const key = JSON.stringify(props);
  useEffect(() => {
    const apply = () => chart.current?.setOption(option(props, host.current ?? document.documentElement), true);
    apply();
    const theme = new MutationObserver(apply); // light/dark switches the root class
    theme.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => theme.disconnect();
  }, [key]);
  return <div ref={host} role="img" aria-label={props.summary} className="h-64 w-full" data-chart={props.type} />;
}

function option(p: ChartProps, el: Element): echarts.EChartsCoreOption {
  const colors = palette(el);
  const text = getComputedStyle(el).getPropertyValue("color") || undefined;
  const calm = typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches;
  const base = {
    color: colors, animation: !calm, textStyle: { color: text },
    // Patterns as well as color for color-blind readers; ECharts' own English aria-label is off, the host's translated summary and the data table speak instead.
    aria: { enabled: true, label: { enabled: false }, decal: { show: true } },
    tooltip: { trigger: p.type === "pie" ? "item" : "axis" },
    legend: p.showLegend ? { bottom: 0, textStyle: { color: text } } : { show: false },
  };
  if (p.type === "pie") {
    return { ...base, series: [{ type: "pie", radius: radius[p.pieStyle], data: p.slices, label: { show: p.showLabels, color: text, formatter: "{b}: {c}" }, avoidLabelOverlap: true,
      itemStyle: p.pieStyle === "classic" ? undefined : { borderColor: "transparent", borderWidth: 2 } }] };
  }
  const categories = p.grid.categories.map((c) => c.label);
  const horizontal = p.type === "bar";
  const category = { type: "category" as const, data: categories, axisLabel: { color: text, interval: 0, hideOverlap: true }, axisLine: { lineStyle: { color: text } } };
  const value = { type: "value" as const, name: p.metric, nameTextStyle: { color: text }, axisLabel: { color: text }, splitLine: { lineStyle: { opacity: 0.25 } } };
  return { ...base, grid: { left: 8, right: 16, top: 28, bottom: p.showLegend ? 40 : 8, containLabel: true },
    xAxis: horizontal ? value : category, yAxis: horizontal ? category : value,
    series: p.grid.series.map((s) => ({ type: p.type === "line" ? "line" : "bar", name: s.name, data: s.values, stack: p.grid.series.length > 1 && p.type !== "line" ? "total" : undefined,
      label: { show: p.showLabels, color: text, position: p.type === "line" ? "top" : undefined }, ...(p.type === "line" ? { symbolSize: 8 } : {}) })) };
}
