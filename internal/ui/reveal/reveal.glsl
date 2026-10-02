precision highp float;

const vec2 LOGO = vec2(1000.0, 666.0);
const vec2 DEVICE = vec2(497.0, 338.0);
const float FLOOR = 462.0;
const float STROKE = 19.5;

const vec2 P0 = vec2(228.0, 462.0);
const vec2 P1 = vec2(228.0, 236.0);
const vec2 P2 = vec2(500.0, 50.0);
const vec2 P3 = vec2(772.0, 236.0);
const vec2 P4 = vec2(772.0, 462.0);

float ease(float x) {
	x = clamp(x, 0.0, 1.0);
	return x * x * (3.0 - 2.0 * x);
}

float outCubic(float x) {
	x = clamp(x, 0.0, 1.0);
	return 1.0 - (1.0 - x) * (1.0 - x) * (1.0 - x);
}

vec2 segment(vec2 p, vec2 a, vec2 b) {
	vec2 ab = b - a;
	float h = clamp(dot(p - a, ab) / dot(ab, ab), 0.0, 1.0);
	return vec2(length(p - a - ab * h), h * length(ab));
}

float arch(vec2 p, out float along) {
	float l0 = length(P1 - P0), l1 = length(P2 - P1), l2 = length(P3 - P2), l3 = length(P4 - P3);
	float total = l0 + l1 + l2 + l3;
	vec2 s = segment(p, P0, P1);
	float d = s.x;
	along = s.y;
	s = segment(p, P1, P2);
	if (s.x < d) { d = s.x; along = l0 + s.y; }
	s = segment(p, P2, P3);
	if (s.x < d) { d = s.x; along = l0 + l1 + s.y; }
	s = segment(p, P3, P4);
	if (s.x < d) { d = s.x; along = l0 + l1 + l2 + s.y; }
	along /= total;
	return d;
}

vec2 archPoint(float f) {
	float l0 = length(P1 - P0), l1 = length(P2 - P1), l2 = length(P3 - P2), l3 = length(P4 - P3);
	float at = f * (l0 + l1 + l2 + l3);
	if (at < l0) return mix(P0, P1, at / l0);
	at -= l0;
	if (at < l1) return mix(P1, P2, at / l1);
	at -= l1;
	if (at < l2) return mix(P2, P3, at / l2);
	at -= l2;
	return mix(P3, P4, clamp(at / l3, 0.0, 1.0));
}

vec3 neon(float s) {
	vec3 c = mix(vec3(1.0, 0.52, 0.0), vec3(1.0, 0.72, 0.04), smoothstep(0.0, 0.3, s));
	c = mix(c, vec3(0.62, 0.86, 0.28), smoothstep(0.3, 0.42, s));
	c = mix(c, vec3(0.0, 0.76, 0.95), smoothstep(0.42, 0.54, s));
	c = mix(c, vec3(0.0, 0.55, 0.99), smoothstep(0.54, 0.85, s));
	return c;
}

float edge(vec2 p, vec2 a, vec2 b) { return (b.x - a.x) * (p.y - a.y) - (b.y - a.y) * (p.x - a.x); }

bool onScreen(vec2 p) {
	vec2 a = vec2(396.0, 224.0), b = vec2(699.0, 207.0), c = vec2(705.0, 432.0), d = vec2(426.0, 446.0);
	return edge(p, a, b) >= 0.0 && edge(p, b, c) >= 0.0 && edge(p, c, d) >= 0.0 && edge(p, d, a) >= 0.0;
}

vec4 logoAt(vec2 q) {
	if (q.x < 0.0 || q.y < 0.0 || q.x > LOGO.x || q.y > LOGO.y) return vec4(0.0);
	vec4 c = texture2D(t0, q / LOGO);
	return vec4(c.rgb, c.a);
}

vec4 device(vec2 q, float theta, float lift, float lit) {
	vec2 dq = q - DEVICE - vec2(0.0, lift);
	float f = 1500.0;
	float ct = cos(theta), st = sin(theta);
	float denom = ct * f - dq.x * st;
	if (denom <= 1.0) return vec4(0.0);
	float x = dq.x * f / denom;
	float y = dq.y * (f + x * st) / f;
	vec2 src = vec2(x, y) + DEVICE;
	if (src.x < 250.0 || src.x > 744.0 || src.y < 196.0 || src.y > FLOOR + 4.0) return vec4(0.0);
	float along;
	if (arch(src, along) < STROKE + 2.5) return vec4(0.0);
	vec4 c = logoAt(src);
	if (c.a <= 0.0) return vec4(0.0);
	if (onScreen(src)) {
		float blue = smoothstep(0.35, 0.55, c.b - c.r);
		float white = smoothstep(0.55, 0.75, min(c.r, min(c.g, c.b)));
		float content = max(blue, white);
		vec3 dark = vec3(0.035, 0.04, 0.055) + 0.04 * (1.0 - src.y / FLOOR);
		vec3 shown = c.rgb * (1.0 + 0.35 * exp(-pow((lit - 0.35) / 0.18, 2.0)));
		c.rgb = mix(c.rgb, mix(dark, shown, lit), content);
	}
	float shade = 1.0 - 0.45 * st * clamp((x + 250.0) / 500.0, 0.0, 1.0);
	float sweep = exp(-pow((src.x - mix(220.0, 820.0, 1.0 - theta / 1.5)) / 55.0, 2.0)) * 0.35 * st;
	c.rgb = c.rgb * shade + sweep;
	return c;
}

void main() {
	vec2 p = pixel();
	float t = u[0];
	float trace = clamp(u[1], 0.0, 1.0);
	float header = ease(u[2]);
	float ready = clamp(u[3], 0.0, 1.0);
	float dark = u[4];
	vec3 bg = vec3(u[5], u[6], u[7]);

	vec2 centre = mix(vec2(u[8], u[9]), vec2(u[11], u[12]), header);
	float scale = mix(u[10], u[13], header);
	vec2 q = (p - centre) / scale + LOGO * 0.5;
	float aa = 1.2 / scale;

	float turn = outCubic((t - 0.25) / 2.0);
	float theta = (1.0 - turn) * 1.5;
	float lift = (1.0 - turn) * 26.0;
	float lit = ease((t - 1.75) / 0.8);

	vec3 col = bg;

	float floorFade = 1.0 - smoothstep(FLOOR, FLOOR + 90.0, q.y);
	if (q.y > FLOOR && floorFade > 0.0) {
		vec2 m = vec2(q.x, 2.0 * FLOOR - q.y + 2.0 * lift);
		vec4 r = device(m, theta, lift, lit);
		col = mix(col, r.rgb, r.a * floorFade * mix(0.16, 0.22, dark));
		float shadow = exp(-pow((q.x - DEVICE.x) / 230.0, 2.0)) * exp(-pow((q.y - FLOOR - 4.0) / 9.0, 2.0));
		col *= 1.0 - shadow * mix(0.18, 0.35, dark) * turn;
	}

	vec4 d = device(q, theta, lift, lit);
	col = mix(col, d.rgb, d.a);

	float along;
	float dist = arch(q, along);
	vec3 glowColour = vec3(0.0);
	float boost = 1.0 + 1.6 * sin(3.14159 * clamp(ready / 0.45, 0.0, 1.0));
	if (trace > 0.0) {
		float shownAt = step(along, trace);
		float body = smoothstep(STROKE + aa, STROKE - aa, dist) * shownAt;
		vec3 nc = neon(along);
		float sheen = 0.82 + 0.18 * smoothstep(STROKE, 0.0, dist);
		col = mix(col, nc * sheen, body);
		float halo = exp(-max(dist - STROKE, 0.0) / mix(9.0, 16.0, dark)) * shownAt * (1.0 - body);
		glowColour += nc * halo * mix(0.16, 0.55, dark) * boost;
		if (trace < 1.0) {
			float tip = length(q - archPoint(trace));
			glowColour += mix(nc, vec3(1.0), 0.6) * exp(-tip / 14.0) * 1.1;
		}
	}

	float word = ease((t - 2.4) / 0.7);
	if (q.y > 480.0 && word > 0.0) {
		vec4 w = logoAt(q + vec2(0.0, (1.0 - word) * 22.0));
		col = mix(col, w.rgb, w.a * word);
	}

	col += glowColour;
	col = mix(col, bg, ease((ready - 0.55) / 0.45));
	col = mix(vec3(0.0), col, outCubic(t / 0.45));
	gl_FragColor = vec4(min(col, vec3(1.0)), 1.0);
}
