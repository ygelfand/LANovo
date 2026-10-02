precision highp float;

float wedge(vec2 q, vec2 a, vec2 b, float w0, float w1, float soft) {
	vec2 d = b - a;
	float t = clamp(dot(q - a, d) / dot(d, d), 0.0, 1.0);
	return clamp((w0 + (w1 - w0) * t - length(q - a - d * t)) / soft + 0.5, 0.0, 1.0);
}

float rounded(vec2 q, vec2 at, vec2 size, float r, float soft) {
	vec2 e = max(max(at + r - q, q - (at + size - r)), 0.0);
	return clamp((r - length(e)) / soft + 0.5, 0.0, 1.0);
}

vec3 ramp3(float t, vec3 a, vec3 b, vec3 c, float mid) {
	return t < mid ? mix(a, b, t / mid) : mix(b, c, (t - mid) / (1.0 - mid));
}

vec3 meter(vec3 col, vec3 back, vec2 q, float fx0, float fy0, float cx, float cy, float theta, float glow, float peak) {
	float S = u[0], fu = u[1], fw = u[2], fh = u[3], rs = u[4];
	if (q.x < floor(fx0) || q.y < floor(fy0) || q.x >= ceil(fx0 + fw) || q.y >= ceil(fy0 + fh)) return col;

	float gr = fw * 0.8;
	float gd = length(q - vec2(cx, fy0 + fh * 1.05));
	col += max(1.0 - gd / gr, 0.0) * glow * vec3(1.0, 186.0 / 255.0, 90.0 / 255.0);
	col = min(col, 1.0);

	vec2 dir = vec2(cos(theta), sin(theta));
	vec2 a = vec2(cx, cy) + dir * rs * 0.25;
	vec2 b = vec2(cx, cy) + dir * rs * 1.02;
	vec2 off = vec2(2.6, 3.6) * fu;
	float k = wedge(q, a + off, b + off, 1.3 * fu, 0.45 * fu, max(3.2 * fu, S));
	col = mix(col, vec3(0.0), k * 61.0 / 255.0);
	k = wedge(q, a, b, 1.3 * fu, 0.45 * fu, S);
	col = mix(col, vec3(18.0 / 255.0), k);

	if (q.y >= floor(fy0 + fh * 0.855)) col = back;

	if (peak > 0.5) {
		vec2 l = vec2(fx0 + fw - 22.0 * fu, fy0 + 22.0 * fu);
		float lr = 5.0 * fu;
		float d = length(q - l);
		if (d < lr) {
			vec3 c = ramp3(d / lr, vec3(1.0), vec3(1.0, 74.0 / 255.0, 74.0 / 255.0), vec3(160.0 / 255.0, 0.0, 0.0), 0.35);
			col = mix(col, c, clamp((lr - d) / S, 0.0, 1.0));
		}
		col = min(col + vec3(1.0, 60.0 / 255.0, 60.0 / 255.0) * 0.6 * max(1.0 - d / (4.0 * lr), 0.0), 1.0);
	}

	float cover = rounded(q, vec2(fx0, fy0), vec2(fw, fh), 8.0 * fu, S);
	vec2 g = vec2(fw * 0.55, fh);
	float t = dot(q - vec2(fx0, fy0), g) / dot(g, g);
	float hi = t < 0.36 ? mix(0.20, 0.05, t / 0.36) : (t < 0.37 ? 0.05 * (0.37 - t) / 0.01 : 0.0);
	float edge = 14.0 * fu;
	float sh = q.y - fy0 < edge ? 0.28 * (1.0 - (q.y - fy0) / edge) : 0.0;
	float alpha = (sh + hi * (1.0 - sh)) * cover;
	return hi * (1.0 - sh) * cover + col * (1.0 - alpha);
}

void main() {
	vec2 p = pixel();
	vec2 q = p * u[0];
	vec3 back = tex(t0, p).rgb;
	vec3 col = meter(back, back, q, u[8], u[9], u[10], u[11], u[12], u[13], u[14]);
	col = meter(col, back, q, u[16], u[17], u[18], u[19], u[20], u[21], u[22]);
	gl_FragColor = vec4(col, 1.0);
}
