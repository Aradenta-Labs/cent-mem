import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCollide,
  forceX,
  forceY,
  Simulation,
  SimulationNodeDatum,
  SimulationLinkDatum,
} from 'd3-force';
import {
  Network,
  ZoomIn,
  ZoomOut,
  Maximize2,
  RefreshCw,
  Eye,
  EyeOff,
  Sparkles,
  AlertTriangle,
} from 'lucide-react';
import { GraphNode, GraphEdge, RelationType } from '../types/memory';
import { fetchGraph } from '../services/api';
import { Button } from './Button';
import { Badge } from './Badge';

export interface KnowledgeGraphViewProps {
  selectedScope: string;
  searchQuery: string;
  onSelectMemory: (id: number) => void;
  refreshKey?: number;
}

interface SimNode extends GraphNode, SimulationNodeDatum {
  activeDegree: number;
}

interface SimLink extends SimulationLinkDatum<SimNode> {
  id: number;
  relation: RelationType;
  suggested: boolean;
  fromId: number;
  toId: number;
}

const ALL_RELATIONS: RelationType[] = [
  'supports',
  'refines',
  'contradicts',
  'depends-on',
  'supersedes',
];

const RELATION_COLORS: Record<RelationType, string> = {
  contradicts: '#ef4444',
  supersedes: '#f97316',
  refines: '#3b82f6',
  supports: '#10b981',
  'depends-on': '#8b5cf6',
};

const SCOPE_PALETTE = [
  '#0d9488', // teal
  '#3b82f6', // blue
  '#8b5cf6', // violet
  '#ec4899', // pink
  '#f59e0b', // amber
  '#10b981', // emerald
  '#ef4444', // red
  '#06b6d4', // cyan
  '#6366f1', // indigo
  '#84cc16', // lime
];

function getNodeRadius(degree: number): number {
  return Math.min(18, 5.5 + Math.sqrt(Math.max(0, degree)) * 2.6);
}

function getTypeColor(type: 'note' | 'fact' | 'log', isDark: boolean): string {
  switch (type) {
    case 'fact':
      return isDark ? '#38bdf8' : '#2563eb';
    case 'log':
      return isDark ? '#a78bfa' : '#7c3aed';
    case 'note':
    default:
      return isDark ? '#14b8a6' : '#0f766e';
  }
}

function readThemePalette() {
  if (typeof window === 'undefined') {
    return {
      isDark: false,
      bg: '#f8fafc',
      surface: '#ffffff',
      border: '#e2e8f0',
      textPrimary: '#0f172a',
      textSecondary: '#475569',
      textMuted: '#64748b',
      accent: '#0f766e',
      dotGrid: 'rgba(148, 163, 184, 0.22)',
    };
  }
  const style = getComputedStyle(document.documentElement);
  const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
  return {
    isDark,
    bg: style.getPropertyValue('--surface-secondary').trim() || (isDark ? '#1e293b' : '#f1f5f9'),
    surface: style.getPropertyValue('--surface-primary').trim() || (isDark ? '#0f172a' : '#ffffff'),
    border: style.getPropertyValue('--border-subtle').trim() || (isDark ? '#334155' : '#e2e8f0'),
    textPrimary: style.getPropertyValue('--text-primary').trim() || (isDark ? '#f8fafc' : '#0f172a'),
    textSecondary: style.getPropertyValue('--text-secondary').trim() || (isDark ? '#cbd5e1' : '#475569'),
    textMuted: style.getPropertyValue('--text-muted').trim() || (isDark ? '#94a3b8' : '#64748b'),
    accent: style.getPropertyValue('--accent-primary').trim() || (isDark ? '#14b8a6' : '#0f766e'),
    dotGrid: isDark ? 'rgba(148, 163, 184, 0.12)' : 'rgba(100, 116, 139, 0.16)',
  };
}

export const KnowledgeGraphView: React.FC<KnowledgeGraphViewProps> = ({
  selectedScope,
  searchQuery,
  onSelectMemory,
  refreshKey,
}) => {
  const [rawNodes, setRawNodes] = useState<GraphNode[]>([]);
  const [rawEdges, setRawEdges] = useState<GraphEdge[]>([]);
  const [totalNodes, setTotalNodes] = useState<number>(0);
  const [truncated, setTruncated] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Controls
  const [colorBy, setColorBy] = useState<'type' | 'scope'>('type');
  const [typeFilter, setTypeFilter] = useState<'all' | 'note' | 'fact' | 'log'>('all');
  const [hideUnlinked, setHideUnlinked] = useState<boolean>(false);
  const [showSuggested, setShowSuggested] = useState<boolean>(true);
  const [activeRelations, setActiveRelations] = useState<Set<RelationType>>(
    () => new Set(ALL_RELATIONS)
  );

  // Hover state for floating tooltip
  const [hoveredNode, setHoveredNode] = useState<SimNode | null>(null);

  // Theme palette synced with data-theme attribute
  const [theme, setTheme] = useState(readThemePalette);

  useEffect(() => {
    const observer = new MutationObserver(() => {
      setTheme(readThemePalette());
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    });
    return () => observer.disconnect();
  }, []);

  // Refs for Canvas & simulation
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const simRef = useRef<Simulation<SimNode, SimLink> | null>(null);
  const simNodesRef = useRef<SimNode[]>([]);
  const simLinksRef = useRef<SimLink[]>([]);
  const posCacheRef = useRef<Map<number, { x: number; y: number }>>(new Map());
  const transformRef = useRef<{ x: number; y: number; k: number }>({ x: 0, y: 0, k: 1 });
  const userInteractedRef = useRef<boolean>(false);
  const hoveredIdRef = useRef<number | null>(null);
  const neighborMapRef = useRef<Map<number, Set<number>>>(new Map());
  const sizeRef = useRef<{ width: number; height: number }>({ width: 900, height: 600 });

  // Drag / pan state
  const dragNodeRef = useRef<SimNode | null>(null);
  const isPanningRef = useRef<boolean>(false);
  const pointerDownRef = useRef<{ x: number; y: number; tx: number; ty: number }>({
    x: 0,
    y: 0,
    tx: 0,
    ty: 0,
  });
  const dragMovedRef = useRef<boolean>(false);

  const loadGraph = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await fetchGraph({
        scope: selectedScope,
        includeSuggested: true,
        limit: 500,
      });
      setRawNodes(res.nodes);
      setRawEdges(res.edges);
      setTotalNodes(res.total_nodes);
      setTruncated(res.truncated);
    } catch (err: any) {
      setError(err.message || 'Failed to load memory graph');
    } finally {
      setIsLoading(false);
    }
  }, [selectedScope]);

  // Clear cached positions when switching scopes so layout re-centers cleanly
  useEffect(() => {
    posCacheRef.current.clear();
    userInteractedRef.current = false;
  }, [selectedScope]);

  useEffect(() => {
    loadGraph();
  }, [loadGraph, refreshKey]);

  // Deterministic scope color map
  const scopeColorMap = useMemo(() => {
    const uniqueScopes = Array.from(new Set(rawNodes.map((n) => n.scope))).sort();
    const map = new Map<string, string>();
    uniqueScopes.forEach((sc, idx) => {
      map.set(sc, SCOPE_PALETTE[idx % SCOPE_PALETTE.length]);
    });
    return map;
  }, [rawNodes]);

  // Compute filtered nodes and edges
  const { filteredNodes, filteredEdges } = useMemo(() => {
    const candidateEdges = rawEdges.filter((e) => {
      if (!showSuggested && e.suggested) return false;
      if (!activeRelations.has(e.relation)) return false;
      return true;
    });

    const inScopeNodes = rawNodes.filter((n) => {
      if (n.external) return false;
      if (typeFilter !== 'all' && n.type !== typeFilter) return false;
      return true;
    });
    const inScopeIds = new Set(inScopeNodes.map((n) => n.id));

    // Include external nodes only if connected to a visible in-scope node via an active edge
    const neededExternalIds = new Set<number>();
    for (const e of candidateEdges) {
      const fromIn = inScopeIds.has(e.from_id);
      const toIn = inScopeIds.has(e.to_id);
      if (fromIn && !toIn) neededExternalIds.add(e.to_id);
      if (toIn && !fromIn) neededExternalIds.add(e.from_id);
    }

    const externalNodes = rawNodes.filter((n) => n.external && neededExternalIds.has(n.id));
    let nodes = [...inScopeNodes, ...externalNodes];
    let nodeIdSet = new Set(nodes.map((n) => n.id));

    // Keep only edges whose endpoints are both present
    let edges = candidateEdges.filter((e) => nodeIdSet.has(e.from_id) && nodeIdSet.has(e.to_id));

    // Compute visible degree
    const degMap = new Map<number, number>();
    for (const e of edges) {
      degMap.set(e.from_id, (degMap.get(e.from_id) || 0) + 1);
      degMap.set(e.to_id, (degMap.get(e.to_id) || 0) + 1);
    }

    if (hideUnlinked) {
      nodes = nodes.filter((n) => (degMap.get(n.id) || 0) > 0);
      nodeIdSet = new Set(nodes.map((n) => n.id));
      edges = edges.filter((e) => nodeIdSet.has(e.from_id) && nodeIdSet.has(e.to_id));
    }

    const nodesWithDegree: SimNode[] = nodes.map((n) => {
      const cached = posCacheRef.current.get(n.id);
      return {
        ...n,
        activeDegree: degMap.get(n.id) || 0,
        x: cached?.x,
        y: cached?.y,
      };
    });

    return {
      filteredNodes: nodesWithDegree,
      filteredEdges: edges,
    };
  }, [rawNodes, rawEdges, showSuggested, activeRelations, typeFilter, hideUnlinked]);

  // Search matching
  const matchedNodeIds = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return null;
    const strippedId = q.startsWith('#') ? q.slice(1) : q;
    const matched = new Set<number>();
    for (const n of filteredNodes) {
      if (
        String(n.id) === strippedId ||
        n.content.toLowerCase().includes(q) ||
        (n.key && n.key.toLowerCase().includes(q)) ||
        n.scope.toLowerCase().includes(q) ||
        n.tags.some((t) => t.toLowerCase().includes(q))
      ) {
        matched.add(n.id);
      }
    }
    return matched;
  }, [filteredNodes, searchQuery]);

  // Draw canvas
  const drawRef = useRef<() => void>(() => {});

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const { width, height } = sizeRef.current;
    const dpr = window.devicePixelRatio || 1;
    const targetW = Math.floor(width * dpr);
    const targetH = Math.floor(height * dpr);
    if (canvas.width !== targetW || canvas.height !== targetH) {
      canvas.width = targetW;
      canvas.height = targetH;
    }

    const { x: tx, y: ty, k: scale } = transformRef.current;
    const nodes = simNodesRef.current;
    const links = simLinksRef.current;
    const hoveredId = hoveredIdRef.current;
    const neighbors = hoveredId !== null ? neighborMapRef.current.get(hoveredId) : undefined;

    ctx.save();
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    // Background fill
    ctx.fillStyle = theme.bg;
    ctx.fillRect(0, 0, width, height);

    // Subtle world-space dot grid
    const gridSpacing = 36 * scale;
    if (gridSpacing >= 12) {
      const offsetX = ((tx % gridSpacing) + gridSpacing) % gridSpacing;
      const offsetY = ((ty % gridSpacing) + gridSpacing) % gridSpacing;
      ctx.fillStyle = theme.dotGrid;
      for (let gx = offsetX; gx < width; gx += gridSpacing) {
        for (let gy = offsetY; gy < height; gy += gridSpacing) {
          ctx.fillRect(gx - 0.75, gy - 0.75, 1.5, 1.5);
        }
      }
    }

    // Apply world transform
    ctx.translate(tx, ty);
    ctx.scale(scale, scale);

    // 1. Draw edges
    for (const link of links) {
      const src = link.source as SimNode;
      const tgt = link.target as SimNode;
      if (
        src.x === undefined ||
        src.y === undefined ||
        tgt.x === undefined ||
        tgt.y === undefined
      ) {
        continue;
      }

      const color = RELATION_COLORS[link.relation] || theme.textMuted;
      const isHighlightedByHover =
        hoveredId !== null && (src.id === hoveredId || tgt.id === hoveredId);
      const isHighlightedBySearch =
        matchedNodeIds !== null && (matchedNodeIds.has(src.id) || matchedNodeIds.has(tgt.id));

      let alpha = 0.52;
      if (hoveredId !== null) {
        alpha = isHighlightedByHover ? 0.95 : 0.08;
      } else if (matchedNodeIds !== null) {
        alpha = isHighlightedBySearch ? 0.8 : 0.1;
      }

      const dx = tgt.x - src.x;
      const dy = tgt.y - src.y;
      const dist = Math.hypot(dx, dy);
      if (dist < 1) continue;

      const srcR = getNodeRadius(src.activeDegree);
      const tgtR = getNodeRadius(tgt.activeDegree);
      const ux = dx / dist;
      const uy = dy / dist;

      const startX = src.x + ux * (srcR + 1.5);
      const startY = src.y + uy * (srcR + 1.5);
      const endX = tgt.x - ux * (tgtR + 4);
      const endY = tgt.y - uy * (tgtR + 4);

      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.strokeStyle = color;
      ctx.lineWidth = isHighlightedByHover ? 2.2 / Math.max(0.7, Math.sqrt(scale)) : 1.4 / Math.max(0.7, Math.sqrt(scale));
      if (link.suggested) {
        ctx.setLineDash([5, 4]);
      }

      ctx.beginPath();
      ctx.moveTo(startX, startY);
      ctx.lineTo(endX, endY);
      ctx.stroke();

      // Directional arrowhead
      const arrowLen = Math.min(8, Math.max(5, 7 / Math.sqrt(scale)));
      const angle = Math.atan2(dy, dx);
      ctx.setLineDash([]);
      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.moveTo(endX, endY);
      ctx.lineTo(
        endX - arrowLen * Math.cos(angle - Math.PI / 6.5),
        endY - arrowLen * Math.sin(angle - Math.PI / 6.5)
      );
      ctx.lineTo(
        endX - arrowLen * Math.cos(angle + Math.PI / 6.5),
        endY - arrowLen * Math.sin(angle + Math.PI / 6.5)
      );
      ctx.closePath();
      ctx.fill();
      ctx.restore();
    }

    // 2. Draw nodes
    for (const node of nodes) {
      if (node.x === undefined || node.y === undefined) continue;

      const r = getNodeRadius(node.activeDegree);
      const isHovered = hoveredId === node.id;
      const isNeighbor = hoveredId !== null && Boolean(neighbors?.has(node.id));
      const isMatched = matchedNodeIds !== null && matchedNodeIds.has(node.id);

      let alpha = 1;
      if (hoveredId !== null) {
        alpha = isHovered || isNeighbor ? 1 : 0.14;
      } else if (matchedNodeIds !== null) {
        alpha = isMatched ? 1 : 0.16;
      }

      const baseColor =
        colorBy === 'scope'
          ? scopeColorMap.get(node.scope) || theme.accent
          : getTypeColor(node.type, theme.isDark);

      ctx.save();
      ctx.globalAlpha = alpha;

      // Search match halo
      if (isMatched) {
        ctx.beginPath();
        ctx.arc(node.x, node.y, r + 4.5, 0, Math.PI * 2);
        ctx.strokeStyle = theme.accent;
        ctx.lineWidth = 2;
        ctx.stroke();
      }

      // Hover outer halo
      if (isHovered) {
        ctx.beginPath();
        ctx.arc(node.x, node.y, r + 4, 0, Math.PI * 2);
        ctx.strokeStyle = baseColor;
        ctx.lineWidth = 2.2;
        ctx.stroke();
      }

      // Main circle
      ctx.beginPath();
      ctx.arc(node.x, node.y, r, 0, Math.PI * 2);
      if (node.external) {
        ctx.fillStyle = theme.surface;
        ctx.fill();
        ctx.setLineDash([3, 2.5]);
        ctx.strokeStyle = baseColor;
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.setLineDash([]);
      } else {
        ctx.fillStyle = baseColor;
        ctx.fill();
        ctx.strokeStyle = theme.surface;
        ctx.lineWidth = isHovered ? 2 : 1.25;
        ctx.stroke();
      }

      // Progressive label reveal
      const showLabel = isHovered || isNeighbor || isMatched || scale >= 0.85;
      if (showLabel) {
        const rawLabel = node.key
          ? `#${node.id} ${node.key}`
          : `#${node.id} ${node.content.replace(/\s+/g, ' ').trim()}`;
        const maxLen = isHovered ? 42 : 24;
        const label = rawLabel.length > maxLen ? rawLabel.slice(0, maxLen) + '…' : rawLabel;

        const fontSize = Math.max(9, Math.min(12, 11 / Math.sqrt(Math.max(0.5, scale))));
        ctx.font = `${isHovered ? 600 : 500} ${fontSize}px system-ui, -apple-system, sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'top';
        ctx.fillStyle = isHovered ? theme.textPrimary : theme.textSecondary;
        ctx.fillText(label, node.x, node.y + r + 4);
      }

      ctx.restore();
    }

    ctx.restore();
  }, [theme, colorBy, scopeColorMap, matchedNodeIds]);

  drawRef.current = draw;

  const fitToView = useCallback((animateNodes?: SimNode[]) => {
    const nodes = animateNodes || simNodesRef.current;
    const { width, height } = sizeRef.current;
    if (!nodes.length || width <= 0 || height <= 0) {
      transformRef.current = { x: width / 2, y: height / 2, k: 1 };
      drawRef.current();
      return;
    }

    let minX = Infinity;
    let maxX = -Infinity;
    let minY = Infinity;
    let maxY = -Infinity;
    for (const n of nodes) {
      if (n.x === undefined || n.y === undefined) continue;
      if (n.x < minX) minX = n.x;
      if (n.x > maxX) maxX = n.x;
      if (n.y < minY) minY = n.y;
      if (n.y > maxY) maxY = n.y;
    }
    if (!Number.isFinite(minX)) return;

    const pad = 64;
    const bw = Math.max(80, maxX - minX);
    const bh = Math.max(80, maxY - minY);
    const k = Math.max(0.22, Math.min(1.6, Math.min((width - pad * 2) / bw, (height - pad * 2) / bh)));
    const cx = (minX + maxX) / 2;
    const cy = (minY + maxY) / 2;

    transformRef.current = {
      x: width / 2 - cx * k,
      y: height / 2 - cy * k,
      k,
    };
    drawRef.current();
  }, []);

  // Build / restart d3-force simulation when filtered data changes
  useEffect(() => {
    if (simRef.current) {
      simRef.current.stop();
      simRef.current = null;
    }

    const simNodes: SimNode[] = filteredNodes.map((n) => ({ ...n }));
    const simLinks: SimLink[] = filteredEdges.map((e) => ({
      id: e.id,
      source: e.from_id,
      target: e.to_id,
      relation: e.relation,
      suggested: e.suggested,
      fromId: e.from_id,
      toId: e.to_id,
    }));

    // Build 1-hop neighbor lookup map
    const nMap = new Map<number, Set<number>>();
    for (const e of filteredEdges) {
      if (!nMap.has(e.from_id)) nMap.set(e.from_id, new Set());
      if (!nMap.has(e.to_id)) nMap.set(e.to_id, new Set());
      nMap.get(e.from_id)!.add(e.to_id);
      nMap.get(e.to_id)!.add(e.from_id);
    }
    neighborMapRef.current = nMap;
    simNodesRef.current = simNodes;
    simLinksRef.current = simLinks;

    if (simNodes.length === 0) {
      drawRef.current();
      return;
    }

    let ticks = 0;
    const sim = forceSimulation<SimNode>(simNodes)
      .force(
        'link',
        forceLink<SimNode, SimLink>(simLinks)
          .id((d) => d.id)
          .distance(95)
          .strength(0.35)
      )
      .force('charge', forceManyBody<SimNode>().strength(-210).distanceMax(420))
      .force(
        'collide',
        forceCollide<SimNode>().radius((d) => getNodeRadius(d.activeDegree) + 10)
      )
      .force('x', forceX<SimNode>(0).strength(0.055))
      .force('y', forceY<SimNode>(0).strength(0.055))
      .alphaDecay(0.03)
      .on('tick', () => {
        ticks++;
        for (const n of simNodes) {
          if (n.x !== undefined && n.y !== undefined) {
            posCacheRef.current.set(n.id, { x: n.x, y: n.y });
          }
        }
        if (ticks === 30 && !userInteractedRef.current) {
          fitToView(simNodes);
        } else {
          drawRef.current();
        }
      });

    simRef.current = sim;

    return () => {
      sim.stop();
      simRef.current = null;
    };
  }, [filteredNodes, filteredEdges, fitToView]);

  // Redraw when visual-only props change (theme, colorBy, searchQuery)
  useEffect(() => {
    draw();
  }, [draw]);

  // Responsive canvas sizing via ResizeObserver
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const updateSize = () => {
      const rect = container.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        const firstInit = sizeRef.current.width === 900 && transformRef.current.x === 0;
        sizeRef.current = { width: rect.width, height: rect.height };
        if (firstInit) {
          transformRef.current = { x: rect.width / 2, y: rect.height / 2, k: 1 };
        }
        drawRef.current();
      }
    };

    updateSize();
    const ro = new ResizeObserver(updateSize);
    ro.observe(container);
    return () => ro.disconnect();
  }, []);

  // Screen -> World coordinate conversion
  const screenToWorld = (sx: number, sy: number) => {
    const { x, y, k } = transformRef.current;
    return {
      wx: (sx - x) / k,
      wy: (sy - y) / k,
    };
  };

  // Hit-test topmost node at screen coordinates
  const hitTestNode = (sx: number, sy: number): SimNode | null => {
    const { wx, wy } = screenToWorld(sx, sy);
    const k = transformRef.current.k;
    const nodes = simNodesRef.current;
    for (let i = nodes.length - 1; i >= 0; i--) {
      const n = nodes[i];
      if (n.x === undefined || n.y === undefined) continue;
      const r = Math.max(getNodeRadius(n.activeDegree) + 5, 11 / k);
      const dx = wx - n.x;
      const dy = wy - n.y;
      if (dx * dx + dy * dy <= r * r) {
        return n;
      }
    }
    return null;
  };

  // Non-passive wheel listener for smooth zoom toward cursor
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      userInteractedRef.current = true;
      const rect = canvas.getBoundingClientRect();
      const sx = e.clientX - rect.left;
      const sy = e.clientY - rect.top;

      const { x, y, k } = transformRef.current;
      const factor = Math.exp(-e.deltaY * 0.0014);
      const nextK = Math.max(0.15, Math.min(4.0, k * factor));
      const nextX = sx - ((sx - x) / k) * nextK;
      const nextY = sy - ((sy - y) / k) * nextK;

      transformRef.current = { x: nextX, y: nextY, k: nextK };
      drawRef.current();
    };

    canvas.addEventListener('wheel', onWheel, { passive: false });
    return () => canvas.removeEventListener('wheel', onWheel);
  }, []);

  const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (e.button !== 0) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const sx = e.clientX - rect.left;
    const sy = e.clientY - rect.top;

    dragMovedRef.current = false;
    pointerDownRef.current = {
      x: sx,
      y: sy,
      tx: transformRef.current.x,
      ty: transformRef.current.y,
    };

    const hit = hitTestNode(sx, sy);
    if (hit) {
      dragNodeRef.current = hit;
      hit.fx = hit.x;
      hit.fy = hit.y;
      simRef.current?.alphaTarget(0.25).restart();
    } else {
      isPanningRef.current = true;
    }
  };

  const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const sx = e.clientX - rect.left;
    const sy = e.clientY - rect.top;

    if (dragNodeRef.current) {
      const dist = Math.hypot(sx - pointerDownRef.current.x, sy - pointerDownRef.current.y);
      if (dist > 4) {
        dragMovedRef.current = true;
        userInteractedRef.current = true;
      }
      const { wx, wy } = screenToWorld(sx, sy);
      dragNodeRef.current.fx = wx;
      dragNodeRef.current.fy = wy;
      drawRef.current();
      return;
    }

    if (isPanningRef.current) {
      const dx = sx - pointerDownRef.current.x;
      const dy = sy - pointerDownRef.current.y;
      if (Math.hypot(dx, dy) > 4) {
        dragMovedRef.current = true;
        userInteractedRef.current = true;
      }
      transformRef.current = {
        ...transformRef.current,
        x: pointerDownRef.current.tx + dx,
        y: pointerDownRef.current.ty + dy,
      };
      drawRef.current();
      return;
    }

    const hit = hitTestNode(sx, sy);
    const nextHoverId = hit ? hit.id : null;
    if (nextHoverId !== hoveredIdRef.current) {
      hoveredIdRef.current = nextHoverId;
      setHoveredNode(hit);
      e.currentTarget.style.cursor = hit ? 'pointer' : 'grab';
      drawRef.current();
    }
  };

  const handleMouseUp = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const draggedNode = dragNodeRef.current;
    if (draggedNode) {
      draggedNode.fx = null;
      draggedNode.fy = null;
      simRef.current?.alphaTarget(0);
      dragNodeRef.current = null;

      if (!dragMovedRef.current) {
        onSelectMemory(draggedNode.id);
      }
    }
    if (isPanningRef.current) {
      isPanningRef.current = false;
      e.currentTarget.style.cursor = hoveredIdRef.current !== null ? 'pointer' : 'grab';
    }
  };

  const handleMouseLeave = () => {
    if (dragNodeRef.current) {
      dragNodeRef.current.fx = null;
      dragNodeRef.current.fy = null;
      simRef.current?.alphaTarget(0);
      dragNodeRef.current = null;
    }
    isPanningRef.current = false;
    if (hoveredIdRef.current !== null) {
      hoveredIdRef.current = null;
      setHoveredNode(null);
      drawRef.current();
    }
  };

  const handleZoomBy = (factor: number) => {
    userInteractedRef.current = true;
    const { width, height } = sizeRef.current;
    const cx = width / 2;
    const cy = height / 2;
    const { x, y, k } = transformRef.current;
    const nextK = Math.max(0.15, Math.min(4.0, k * factor));
    transformRef.current = {
      x: cx - ((cx - x) / k) * nextK,
      y: cy - ((cy - y) / k) * nextK,
      k: nextK,
    };
    drawRef.current();
  };

  const toggleRelation = (rel: RelationType) => {
    setActiveRelations((prev) => {
      const next = new Set(prev);
      if (next.has(rel)) {
        next.delete(rel);
      } else {
        next.add(rel);
      }
      return next;
    });
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-3)',
        height: 'calc(100vh - 140px)',
        minHeight: '520px',
      }}
    >
      {/* Top Controls Bar */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 'var(--space-3)',
          padding: 'var(--space-3) var(--space-4)',
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
        }}
      >
        {/* Left: Title & live counters */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)', flexWrap: 'wrap' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-2)',
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            <Network size={17} color="var(--accent-primary)" />
            <span>Memory Graph</span>
          </div>

          <Badge variant="neutral" size="sm">
            {filteredNodes.length} {filteredNodes.length === 1 ? 'node' : 'nodes'} · {filteredEdges.length}{' '}
            {filteredEdges.length === 1 ? 'edge' : 'edges'}
          </Badge>

          {matchedNodeIds !== null && (
            <Badge variant="accent" size="sm">
              {matchedNodeIds.size} search {matchedNodeIds.size === 1 ? 'match' : 'matches'}
            </Badge>
          )}

          {truncated && (
            <Badge variant="warning" size="sm" title="Narrow by scope or hide unlinked nodes to explore smaller subgraphs">
              <AlertTriangle size={11} />
              <span>Showing 500 of {totalNodes} nodes — narrow by scope</span>
            </Badge>
          )}
        </div>

        {/* Right: Filters & Canvas Controls */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexWrap: 'wrap' }}>
          {/* Color By Switcher */}
          <div
            role="group"
            aria-label="Color nodes by"
            style={{
              display: 'inline-flex',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-sm)',
              overflow: 'hidden',
              backgroundColor: 'var(--surface-secondary)',
            }}
          >
            {(['type', 'scope'] as const).map((mode) => {
              const active = colorBy === mode;
              return (
                <button
                  key={mode}
                  type="button"
                  onClick={() => setColorBy(mode)}
                  style={{
                    border: 'none',
                    padding: '4px 9px',
                    fontSize: '11px',
                    fontWeight: active ? 600 : 500,
                    cursor: 'pointer',
                    backgroundColor: active ? 'var(--surface-primary)' : 'transparent',
                    color: active ? 'var(--accent-primary)' : 'var(--text-secondary)',
                  }}
                >
                  {mode === 'type' ? 'By Type' : 'By Scope'}
                </button>
              );
            })}
          </div>

          {/* Memory Type Filter */}
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value as 'all' | 'note' | 'fact' | 'log')}
            aria-label="Filter by memory type"
            style={{
              height: '28px',
              padding: '0 var(--space-2)',
              borderRadius: 'var(--radius-sm)',
              border: '1px solid var(--border-default)',
              backgroundColor: 'var(--surface-primary)',
              color: 'var(--text-primary)',
              fontSize: '11px',
              cursor: 'pointer',
            }}
          >
            <option value="all">All Types</option>
            <option value="note">Notes</option>
            <option value="fact">Facts</option>
            <option value="log">Logs</option>
          </select>

          {/* Toggle: Hide Unlinked */}
          <button
            type="button"
            onClick={() => setHideUnlinked((v) => !v)}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '5px',
              height: '28px',
              padding: '0 8px',
              fontSize: '11px',
              fontWeight: 500,
              borderRadius: 'var(--radius-sm)',
              border: `1px solid ${hideUnlinked ? 'var(--accent-border)' : 'var(--border-default)'}`,
              backgroundColor: hideUnlinked ? 'var(--accent-lightest)' : 'var(--surface-primary)',
              color: hideUnlinked ? 'var(--accent-primary)' : 'var(--text-secondary)',
              cursor: 'pointer',
            }}
          >
            {hideUnlinked ? <EyeOff size={12} /> : <Eye size={12} />}
            <span>Hide unlinked</span>
          </button>

          {/* Toggle: Auto-Suggested Links */}
          <button
            type="button"
            onClick={() => setShowSuggested((v) => !v)}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '5px',
              height: '28px',
              padding: '0 8px',
              fontSize: '11px',
              fontWeight: 500,
              borderRadius: 'var(--radius-sm)',
              border: `1px solid ${showSuggested ? 'var(--color-warning-border)' : 'var(--border-default)'}`,
              backgroundColor: showSuggested ? 'var(--color-warning-bg)' : 'var(--surface-primary)',
              color: showSuggested ? 'var(--color-warning-text)' : 'var(--text-secondary)',
              cursor: 'pointer',
            }}
          >
            <Sparkles size={12} />
            <span>Auto-suggested</span>
          </button>

          {/* Zoom & Fit Controls */}
          <div style={{ display: 'inline-flex', alignItems: 'center', gap: '2px', marginLeft: '4px' }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleZoomBy(0.8)}
              title="Zoom out"
              aria-label="Zoom out"
              style={{ padding: '4px' }}
            >
              <ZoomOut size={14} />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleZoomBy(1.25)}
              title="Zoom in"
              aria-label="Zoom in"
              style={{ padding: '4px' }}
            >
              <ZoomIn size={14} />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                userInteractedRef.current = false;
                fitToView();
              }}
              title="Fit graph to view"
              aria-label="Fit graph to view"
              style={{ padding: '4px' }}
            >
              <Maximize2 size={13} />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={loadGraph}
              title="Refresh graph"
              aria-label="Refresh graph"
              style={{ padding: '4px' }}
            >
              <RefreshCw size={13} className={isLoading ? 'animate-spin' : ''} />
            </Button>
          </div>
        </div>
      </div>

      {/* Main Canvas Stage */}
      <div
        ref={containerRef}
        style={{
          position: 'relative',
          flex: 1,
          minHeight: 0,
          backgroundColor: 'var(--surface-secondary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
          overflow: 'hidden',
        }}
      >
        {isLoading && rawNodes.length === 0 ? (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--text-muted)',
              fontSize: 'var(--text-xs)',
            }}
          >
            Loading memory graph...
          </div>
        ) : error ? (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 'var(--space-2)',
              color: 'var(--color-error-text)',
              fontSize: 'var(--text-xs)',
            }}
          >
            <span>{error}</span>
            <Button variant="secondary" size="sm" onClick={loadGraph}>
              Retry
            </Button>
          </div>
        ) : filteredNodes.length === 0 ? (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 'var(--space-2)',
              color: 'var(--text-muted)',
              fontSize: 'var(--text-xs)',
              textAlign: 'center',
              padding: 'var(--space-6)',
            }}
          >
            <Network size={28} color="var(--text-muted)" />
            <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
              No memory nodes match the current filters
            </div>
            <div>
              {hideUnlinked
                ? 'Try turning off "Hide unlinked" or switching to a broader scope.'
                : 'Store memories or switch scopes to explore the knowledge graph.'}
            </div>
          </div>
        ) : null}

        <canvas
          ref={canvasRef}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onMouseLeave={handleMouseLeave}
          style={{
            width: '100%',
            height: '100%',
            display: 'block',
            cursor: 'grab',
          }}
        />

        {/* Bottom-left Floating Relation & Node Legend */}
        <div
          style={{
            position: 'absolute',
            left: 'var(--space-3)',
            bottom: 'var(--space-3)',
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            gap: '6px',
            padding: '6px 10px',
            backgroundColor: 'var(--surface-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            boxShadow: 'var(--shadow-sm)',
            maxWidth: 'calc(100% - 24px)',
          }}
        >
          {ALL_RELATIONS.map((rel) => {
            const active = activeRelations.has(rel);
            const color = RELATION_COLORS[rel];
            return (
              <button
                key={rel}
                type="button"
                onClick={() => toggleRelation(rel)}
                title={`Toggle ${rel} edges`}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '5px',
                  padding: '2px 7px',
                  borderRadius: 'var(--radius-pill)',
                  border: `1px solid ${active ? color : 'var(--border-subtle)'}`,
                  backgroundColor: active ? `${color}18` : 'transparent',
                  color: active ? 'var(--text-primary)' : 'var(--text-muted)',
                  fontSize: '10px',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.03em',
                  cursor: 'pointer',
                  opacity: active ? 1 : 0.55,
                }}
              >
                <span
                  style={{
                    width: '7px',
                    height: '7px',
                    borderRadius: '50%',
                    backgroundColor: color,
                  }}
                />
                <span>{rel}</span>
              </button>
            );
          })}

          <span
            style={{
              fontSize: '10px',
              color: 'var(--text-muted)',
              paddingLeft: '6px',
              borderLeft: '1px solid var(--border-subtle)',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '8px',
            }}
          >
            <span>Solid: confirmed</span>
            <span>Dashed: suggested</span>
            {colorBy === 'type' && (
              <>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span style={{ width: '7px', height: '7px', borderRadius: '50%', backgroundColor: getTypeColor('note', theme.isDark) }} />
                  note
                </span>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span style={{ width: '7px', height: '7px', borderRadius: '50%', backgroundColor: getTypeColor('fact', theme.isDark) }} />
                  fact
                </span>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span style={{ width: '7px', height: '7px', borderRadius: '50%', backgroundColor: getTypeColor('log', theme.isDark) }} />
                  log
                </span>
              </>
            )}
          </span>
        </div>

        {/* Floating Hover Node Preview Tooltip */}
        {hoveredNode && (
          <div
            style={{
              position: 'absolute',
              top: 'var(--space-3)',
              right: 'var(--space-3)',
              width: '300px',
              maxWidth: 'calc(100% - 24px)',
              backgroundColor: 'var(--surface-primary)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-md)',
              padding: 'var(--space-3)',
              boxShadow: 'var(--shadow-md)',
              pointerEvents: 'none',
              display: 'flex',
              flexDirection: 'column',
              gap: '6px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '6px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px', fontWeight: 700, color: 'var(--accent-primary)' }}>
                  #{hoveredNode.id}
                </span>
                <Badge variant={hoveredNode.type === 'fact' ? 'accent' : 'neutral'} size="sm">
                  {hoveredNode.type}
                </Badge>
                {hoveredNode.external && (
                  <Badge variant="warning" size="sm">
                    External
                  </Badge>
                )}
              </div>
              <span style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                {hoveredNode.activeDegree} {hoveredNode.activeDegree === 1 ? 'link' : 'links'}
              </span>
            </div>

            <div style={{ fontSize: '10px', fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
              {hoveredNode.scope}
            </div>

            {hoveredNode.key && (
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                {hoveredNode.key}
              </div>
            )}

            <div
              style={{
                fontSize: 'var(--text-xs)',
                color: 'var(--text-secondary)',
                lineHeight: 1.45,
                display: '-webkit-box',
                WebkitLineClamp: 3,
                WebkitBoxOrient: 'vertical',
                overflow: 'hidden',
                wordBreak: 'break-word',
              }}
            >
              {hoveredNode.content}
            </div>

            <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>
              Click node to inspect details & relationships
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
