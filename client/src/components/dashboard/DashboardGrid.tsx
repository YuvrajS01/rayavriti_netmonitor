import { Component, Suspense, useId, useRef, useState, type DragEvent, type ErrorInfo, type ReactNode } from 'react';
import type { DashboardLayout, DashboardWidget } from '../../api/types';
import IconButton from '../ui/IconButton';
import { HEIGHT_LABEL, HEIGHT_STEPS, WIDTH_LABEL, WIDTH_PRESETS, moveWidget, removeWidget, updateWidget } from './layout';
import { widgetMeta, widgetTitle } from './widgets/catalog';
import { widgetComponent } from './widgets/registry';

/*
 * Tailwind needs literal class names. Wide screens use the 12-column grid;
 * medium screens halve it (6 columns); small screens stack everything.
 */
const SPAN: Record<number, string> = {
  1: 'md:col-span-1 xl:col-span-1',
  2: 'md:col-span-1 xl:col-span-2',
  3: 'md:col-span-3 xl:col-span-3',
  4: 'md:col-span-3 xl:col-span-4',
  5: 'md:col-span-3 xl:col-span-5',
  6: 'md:col-span-3 xl:col-span-6',
  7: 'md:col-span-6 xl:col-span-7',
  8: 'md:col-span-6 xl:col-span-8',
  9: 'md:col-span-6 xl:col-span-9',
  10: 'md:col-span-6 xl:col-span-10',
  11: 'md:col-span-6 xl:col-span-11',
  12: 'md:col-span-6 xl:col-span-12',
};
/** Height steps are minimums; a row grows to its tallest widget. */
const MIN_HEIGHT: Record<number, string> = { 1: '', 2: 'min-h-72', 3: 'min-h-[28rem]', 4: 'min-h-[38rem]' };

interface Props {
  layout: DashboardLayout;
  editing: boolean;
  onChange?: (layout: DashboardLayout) => void;
  onConfigure?: (widget: DashboardWidget) => void;
}

export default function DashboardGrid({ layout, editing, onChange, onConfigure }: Props) {
  const [announcement, setAnnouncement] = useState('');
  const [dragId, setDragId] = useState<string | null>(null);
  const [overId, setOverId] = useState<string | null>(null);
  const cellRefs = useRef(new Map<string, HTMLDivElement>());

  const change = (next: DashboardLayout, message: string) => {
    onChange?.(next);
    setAnnouncement(message);
  };
  const move = (w: DashboardWidget, to: number) => {
    const next = moveWidget(layout, w.id, to);
    const at = next.widgets.findIndex((x) => x.id === w.id);
    change(next, `${widgetTitle(w.type, w.config)} moved to position ${at + 1} of ${next.widgets.length}.`);
  };

  const onDrop = (e: DragEvent, target: DashboardWidget) => {
    e.preventDefault();
    const id = dragId ?? e.dataTransfer.getData('text/plain');
    setDragId(null);
    setOverId(null);
    const dragged = layout.widgets.find((w) => w.id === id);
    if (!dragged || dragged.id === target.id) return;
    move(dragged, layout.widgets.findIndex((w) => w.id === target.id));
  };

  return (
    <>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-6 xl:grid-cols-12">
        {layout.widgets.map((widget, index) => {
          const Widget = widgetComponent(widget.type);
          const title = widgetTitle(widget.type, widget.config);
          return (
            <div
              key={widget.id}
              ref={(el) => {
                if (el) cellRefs.current.set(widget.id, el);
                else cellRefs.current.delete(widget.id);
              }}
              className={`flex min-w-0 flex-col ${SPAN[widget.w] ?? SPAN[12]} ${MIN_HEIGHT[widget.h] ?? ''} ${
                editing ? `rounded-lg outline-2 outline-offset-4 ${overId === widget.id && dragId !== widget.id ? 'outline outline-primary' : 'outline-dashed outline-line-strong'} ${dragId === widget.id ? 'opacity-50' : ''}` : ''
              }`}
              onDragOver={editing ? (e) => { e.preventDefault(); setOverId(widget.id); } : undefined}
              onDragLeave={editing ? () => setOverId((cur) => (cur === widget.id ? null : cur)) : undefined}
              onDrop={editing ? (e) => onDrop(e, widget) : undefined}
            >
              {editing && (
                <WidgetToolbar
                  widget={widget}
                  title={title}
                  index={index}
                  count={layout.widgets.length}
                  onDragStart={(e) => {
                    e.dataTransfer.effectAllowed = 'move';
                    e.dataTransfer.setData('text/plain', widget.id);
                    const cell = cellRefs.current.get(widget.id);
                    if (cell) e.dataTransfer.setDragImage(cell, 24, 24);
                    setDragId(widget.id);
                  }}
                  onDragEnd={() => {
                    setDragId(null);
                    setOverId(null);
                  }}
                  onMove={(to) => move(widget, to)}
                  onResize={(patch) => change(updateWidget(layout, widget.id, patch), `${title} resized.`)}
                  onConfigure={onConfigure ? () => onConfigure(widget) : undefined}
                  onRemove={() => change(removeWidget(layout, widget.id), `${title} removed.`)}
                />
              )}
              {/* While editing, widgets are inert so a click can't navigate away mid-edit. */}
              <div className="flex min-h-0 flex-1 flex-col [&>*]:flex-1" inert={editing || undefined}>
                <WidgetBoundary title={title}>
                  <Suspense fallback={<WidgetSkeleton />}>
                    <Widget widget={widget} />
                  </Suspense>
                </WidgetBoundary>
              </div>
            </div>
          );
        })}
      </div>
      <p className="sr-only" aria-live="polite">{announcement}</p>
    </>
  );
}

interface ToolbarProps {
  widget: DashboardWidget;
  title: string;
  index: number;
  count: number;
  onDragStart: (e: DragEvent) => void;
  onDragEnd: () => void;
  onMove: (to: number) => void;
  onResize: (patch: { w?: number; h?: number }) => void;
  onConfigure?: () => void;
  onRemove: () => void;
}

function WidgetToolbar({ widget, title, index, count, onDragStart, onDragEnd, onMove, onResize, onConfigure, onRemove }: ToolbarProps) {
  const widthId = useId();
  const heightId = useId();
  const selectClass = 'h-8 cursor-pointer rounded-md border border-line bg-inset px-2 text-xs text-fg';
  const widths: number[] = WIDTH_PRESETS.includes(widget.w as never) ? [...WIDTH_PRESETS] : [...WIDTH_PRESETS, widget.w].sort((a, b) => a - b);
  return (
    <div className="mb-2 flex flex-wrap items-center gap-1.5 rounded-md border border-line bg-surface-2 px-1.5 py-1">
      <span
        draggable
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        title="Drag to reorder"
        className="material-symbols-outlined cursor-grab select-none px-1 text-[20px] text-fg-muted active:cursor-grabbing"
        aria-hidden="true"
      >
        drag_indicator
      </span>
      <span className="mr-auto min-w-0 truncate text-xs font-semibold text-fg">{title}</span>
      <label htmlFor={widthId} className="sr-only">Width of {title}</label>
      <select id={widthId} className={selectClass} value={widget.w} onChange={(e) => onResize({ w: Number(e.target.value) })}>
        {widths.map((w) => <option key={w} value={w}>{WIDTH_LABEL[w] ?? `${w} of 12`}</option>)}
      </select>
      <label htmlFor={heightId} className="sr-only">Height of {title}</label>
      <select id={heightId} className={selectClass} value={widget.h} onChange={(e) => onResize({ h: Number(e.target.value) })}>
        {HEIGHT_STEPS.map((h) => <option key={h} value={h}>{HEIGHT_LABEL[h]}</option>)}
      </select>
      {onConfigure && widgetMeta(widget.type)?.configurable && <IconButton icon="tune" label={`Settings for ${title}`} size="sm" onClick={onConfigure} />}
      <IconButton icon="arrow_back" label={`Move ${title} earlier`} size="sm" disabled={index === 0} onClick={() => onMove(index - 1)} />
      <IconButton icon="arrow_forward" label={`Move ${title} later`} size="sm" disabled={index === count - 1} onClick={() => onMove(index + 1)} />
      <IconButton icon="delete" label={`Remove ${title}`} size="sm" variant="danger" onClick={onRemove} />
    </div>
  );
}

function WidgetSkeleton() {
  return <div role="status" aria-label="Loading widget" className="h-full min-h-28 animate-pulse rounded-lg border border-line bg-surface-1" />;
}

/** Keeps one broken widget from taking the dashboard down with it. */
class WidgetBoundary extends Component<{ title: string; children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`[dashboard] widget "${this.props.title}" failed:`, error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <div role="alert" className="flex h-full flex-col items-center justify-center gap-3 rounded-lg border border-line bg-surface-1 p-5 text-center">
        <p className="text-sm text-fg-muted">{this.props.title} couldn’t be displayed.</p>
        <button type="button" className="text-sm text-primary underline" onClick={() => this.setState({ failed: false })}>Try again</button>
      </div>
    );
  }
}
