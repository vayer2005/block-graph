import { Canvas } from "@react-three/fiber";
import { Line, MapControls, OrthographicCamera, Instance, Instances } from "@react-three/drei";
import { useMemo, useState } from "react";
import * as THREE from "three";
import type { PlacedEdge, PlacedPoint } from "./types";

const COL_PERIPHERAL = "#b4aee8";
const COL_CENTER_BLUE = "#4a9eff";

type Props = {
  title: string;
  points: PlacedPoint[];
  edges: PlacedEdge[];
};

export function ScenePanel({ title, points, edges }: Props) {
  const [hover, setHover] = useState<string | null>(null);

  const peripheralColor = useMemo(() => new THREE.Color(COL_PERIPHERAL), []);

  const centers = useMemo(() => points.filter((p) => p.role === "center"), [points]);
  const peripherals = useMemo(() => points.filter((p) => p.role === "peripheral"), [points]);

  return (
    <div className="panel">
      <h2 className="panel-title">{title}</h2>
      <div className="canvas-wrap">
        {hover && (
          <pre className="hover-tip hover-tip-overlay" aria-live="polite">
            {hover}
          </pre>
        )}
        <Canvas orthographic dpr={[1, 2]} gl={{ antialias: true, localClippingEnabled: false }}>
          <color attach="background" args={["#2a2a2e"]} />
          <OrthographicCamera makeDefault position={[0, 0, 22]} zoom={22} near={0.1} far={200} />
          <ambientLight intensity={0.55} />
          <directionalLight position={[4, 6, 8]} intensity={0.9} />
          <directionalLight position={[-5, -2, 4]} intensity={0.35} />
          <MapControls
            makeDefault
            enableRotate={false}
            enableDamping
            dampingFactor={0.08}
            screenSpacePanning
          />

          {/* Edges first (behind nodes) */}
          <group renderOrder={0}>
            {edges.map((e, i) => (
              <Line
                key={i}
                points={[e.from, e.to]}
                color="#8b93a8"
                lineWidth={1}
                depthTest
                transparent
                opacity={0.95}
              />
            ))}
          </group>

          {/* Blue center node(s) — larger */}
          <group renderOrder={2}>
            {centers.map((p, i) => (
              <mesh
                key={`c-${p.txid}-${i}`}
                position={p.position}
                onPointerOver={(e) => {
                  e.stopPropagation();
                  setHover(p.label);
                  document.body.style.cursor = "pointer";
                }}
                onPointerOut={() => {
                  setHover(null);
                  document.body.style.cursor = "auto";
                }}
              >
                <sphereGeometry args={[0.24, 22, 22]} />
                <meshStandardMaterial color={COL_CENTER_BLUE} roughness={0.35} metalness={0.15} />
              </mesh>
            ))}
          </group>

          {/* Peripheral nodes — smaller */}
          {peripherals.length > 0 && (
            <group renderOrder={3}>
              <Instances limit={peripherals.length} range={peripherals.length}>
                <sphereGeometry args={[0.125, 14, 14]} />
                <meshStandardMaterial roughness={0.4} metalness={0.1} />
                {peripherals.map((p, i) => (
                  <Instance
                    key={`${p.txid}-${i}`}
                    position={p.position}
                    color={peripheralColor}
                    onPointerOver={(e) => {
                      e.stopPropagation();
                      setHover(p.label);
                      document.body.style.cursor = "pointer";
                    }}
                    onPointerOut={() => {
                      setHover(null);
                      document.body.style.cursor = "auto";
                    }}
                  />
                ))}
              </Instances>
            </group>
          )}
        </Canvas>
      </div>
    </div>
  );
}
