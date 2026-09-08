import React, { useState } from 'react';
import { MemoryLinkWithContent, RelationType } from '../types/memory';
import { Maximize2, Minimize2 } from 'lucide-react';

export interface MemoryGraphViewProps {
  currentMemoryId: number;
  currentContent?: string;
  outgoing: MemoryLinkWithContent[];
  incoming: MemoryLinkWithContent[];
  onSelectMemory?: (id: number) => void;
  height?: number;
}

const RELATION_COLORS: Record<RelationType, string> = {
  'contradicts': '#ef4444',
  'supersedes': '#f97316',
  'refines': '#3b82f6',
  'supports': '#10b981',
  'depends-on': '#8b5cf6',
};

export const MemoryGraphView: React.FC<MemoryGraphViewProps> = ({
  currentMemoryId,
  currentContent,
  outgoing,
  incoming,
  onSelectMemory,
  height = 240,
}) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const [hoveredNode, setHoveredNode] = useState<{ id: number; text: string; relation: string } | null>(null);

  const totalLinks = outgoing.length + incoming.length;

  if (totalLinks === 0) {
    return (
      <div
        style={{
          height: `${height}px`,
          backgroundColor: 'var(--surface-secondary)',
          border: '1px dashed var(--border-subtle)',
          borderRadius: 'var(--radius-md)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 'var(--space-2)',
          color: 'var(--text-muted)',
          fontSize: 'var(--text-xs)',
        }}
      >
        <span>No connected memory relationships.</span>
      </div>
    );
  }

  const svgHeight = isExpanded ? 480 : height;
  const svgWidth = 460;
  const centerX = svgWidth / 2;
  const centerY = svgHeight / 2;

  // Calculate coordinates for incoming nodes (left side)
  const inCoords = incoming.map((link, idx) => {
    const total = incoming.length;
    const step = total === 1 ? 0 : (svgHeight - 100) / (total - 1);
    const y = total === 1 ? centerY : 50 + idx * step;
    return {
      x: 70,
      y,
      link,
      id: link.from_id,
      text: link.source_content || `Memory #${link.from_id}`,
      relation: link.relation,
      suggested: link.suggested,
      isIncoming: true,
    };
  });

  // Calculate coordinates for outgoing nodes (right side)
  const outCoords = outgoing.map((link, idx) => {
    const total = outgoing.length;
    const step = total === 1 ? 0 : (svgHeight - 100) / (total - 1);
    const y = total === 1 ? centerY : 50 + idx * step;
    return {
      x: svgWidth - 70,
      y,
      link,
      id: link.to_id,
      text: link.target_content || `Memory #${link.to_id}`,
      relation: link.relation,
      suggested: link.suggested,
      isIncoming: false,
    };
  });

  return (
    <div
      style={{
        position: 'relative',
        backgroundColor: 'var(--surface-secondary)',
        border: '1px solid var(--border-subtle)',
        borderRadius: 'var(--radius-md)',
        overflow: 'hidden',
      }}
    >
      {/* Header controls */}
      <div
        style={{
          position: 'absolute',
          top: 'var(--space-2)',
          right: 'var(--space-2)',
          zIndex: 10,
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-1)',
        }}
      >
        <button
          type="button"
          onClick={() => setIsExpanded(!isExpanded)}
          style={{
            background: 'var(--surface-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-sm)',
            cursor: 'pointer',
            padding: '4px',
            color: 'var(--text-secondary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
          title={isExpanded ? 'Collapse Topology' : 'Expand Topology'}
        >
          {isExpanded ? <Minimize2 size={13} /> : <Maximize2 size={13} />}
        </button>
      </div>

      <svg
        width="100%"
        height={svgHeight}
        viewBox={`0 0 ${svgWidth} ${svgHeight}`}
        style={{ display: 'block' }}
      >
        <defs>
          {/* Arrow markers for relations */}
          {(['contradicts', 'supersedes', 'refines', 'supports', 'depends-on'] as RelationType[]).map((rel) => (
            <marker
              key={rel}
              id={`arrow-${rel}`}
              viewBox="0 0 10 10"
              refX="16"
              refY="5"
              markerWidth="6"
              markerHeight="6"
              orient="auto-start-reverse"
            >
              <path d="M 0 1.5 L 10 5 L 0 8.5 z" fill={RELATION_COLORS[rel]} />
            </marker>
          ))}
        </defs>

        {/* Edges from incoming nodes to center */}
        {inCoords.map((item, idx) => {
          const color = RELATION_COLORS[item.relation] || 'var(--text-muted)';
          return (
            <g key={`in-edge-${idx}`}>
              <path
                d={`M ${item.x} ${item.y} C ${centerX - 40} ${item.y}, ${centerX - 40} ${centerY}, ${centerX} ${centerY}`}
                fill="none"
                stroke={color}
                strokeWidth={item.suggested ? 1.5 : 2}
                strokeDasharray={item.suggested ? '4 4' : undefined}
                markerEnd={`url(#arrow-${item.relation})`}
                opacity={0.8}
              />
            </g>
          );
        })}

        {/* Edges from center to outgoing nodes */}
        {outCoords.map((item, idx) => {
          const color = RELATION_COLORS[item.relation] || 'var(--text-muted)';
          return (
            <g key={`out-edge-${idx}`}>
              <path
                d={`M ${centerX} ${centerY} C ${centerX + 40} ${centerY}, ${centerX + 40} ${item.y}, ${item.x} ${item.y}`}
                fill="none"
                stroke={color}
                strokeWidth={item.suggested ? 1.5 : 2}
                strokeDasharray={item.suggested ? '4 4' : undefined}
                markerEnd={`url(#arrow-${item.relation})`}
                opacity={0.8}
              />
            </g>
          );
        })}

        {/* Center Node: Current Memory */}
        <g transform={`translate(${centerX}, ${centerY})`}>
          <title>{currentContent || `#${currentMemoryId}`}</title>
          <circle
            r={24}
            fill="var(--surface-primary)"
            stroke="var(--accent-primary)"
            strokeWidth={3}
          />
          <text
            textAnchor="middle"
            dy="4"
            fontFamily="var(--font-mono)"
            fontSize="10"
            fontWeight="700"
            fill="var(--accent-primary)"
          >
            #{currentMemoryId}
          </text>
        </g>

        {/* Incoming Nodes */}
        {inCoords.map((item) => {
          const color = RELATION_COLORS[item.relation];
          return (
            <g
              key={`in-node-${item.id}`}
              transform={`translate(${item.x}, ${item.y})`}
              style={{ cursor: onSelectMemory ? 'pointer' : 'default' }}
              onClick={() => onSelectMemory?.(item.id)}
              onMouseEnter={() => setHoveredNode({ id: item.id, text: item.text, relation: item.relation })}
              onMouseLeave={() => setHoveredNode(null)}
            >
              <circle
                r={16}
                fill="var(--surface-primary)"
                stroke={color}
                strokeWidth={1.5}
                strokeDasharray={item.suggested ? '3 3' : undefined}
              />
              <text
                textAnchor="middle"
                dy="3.5"
                fontFamily="var(--font-mono)"
                fontSize="9"
                fontWeight="600"
                fill="var(--text-secondary)"
              >
                #{item.id}
              </text>
            </g>
          );
        })}

        {/* Outgoing Nodes */}
        {outCoords.map((item) => {
          const color = RELATION_COLORS[item.relation];
          return (
            <g
              key={`out-node-${item.id}`}
              transform={`translate(${item.x}, ${item.y})`}
              style={{ cursor: onSelectMemory ? 'pointer' : 'default' }}
              onClick={() => onSelectMemory?.(item.id)}
              onMouseEnter={() => setHoveredNode({ id: item.id, text: item.text, relation: item.relation })}
              onMouseLeave={() => setHoveredNode(null)}
            >
              <circle
                r={16}
                fill="var(--surface-primary)"
                stroke={color}
                strokeWidth={1.5}
                strokeDasharray={item.suggested ? '3 3' : undefined}
              />
              <text
                textAnchor="middle"
                dy="3.5"
                fontFamily="var(--font-mono)"
                fontSize="9"
                fontWeight="600"
                fill="var(--text-secondary)"
              >
                #{item.id}
              </text>
            </g>
          );
        })}
      </svg>

      {/* Floating tooltip on hover */}
      {hoveredNode && (
        <div
          style={{
            position: 'absolute',
            bottom: 'var(--space-2)',
            left: 'var(--space-2)',
            right: 'var(--space-2)',
            backgroundColor: 'var(--surface-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-sm)',
            padding: 'var(--space-2) var(--space-3)',
            boxShadow: 'var(--shadow-md)',
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-2)',
            fontSize: '11px',
            pointerEvents: 'none',
          }}
        >
          <span
            style={{
              fontFamily: 'var(--font-mono)',
              fontWeight: 700,
              color: RELATION_COLORS[hoveredNode.relation as RelationType] || 'var(--text-primary)',
              textTransform: 'uppercase',
              fontSize: '10px',
            }}
          >
            {hoveredNode.relation}
          </span>
          <span
            style={{
              color: 'var(--text-primary)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            #{hoveredNode.id}: {hoveredNode.text}
          </span>
        </div>
      )}
    </div>
  );
};
