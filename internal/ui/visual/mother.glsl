float rounded(highp vec2 p, highp vec2 at, highp vec2 size, float r) {
	highp vec2 e = max(max(at + r - p, p - (at + size - r)), 0.0);
	return clamp(r - float(length(e)) + 0.5, 0.0, 1.0);
}

highp float wave(float i, float row) {
	vec4 c = texture2D(t3, vec2((clamp(i, 0.0, 199.0) + 0.5) / 256.0, (row + 0.5) / 4.0));
	return dec16(c.rg) * 2.0 - 1.0;
}

float trace(highp vec2 p, float row) {
	highp float fx0 = u[14], fx1 = u[15], fy = u[16], amp = u[17];
	highp float fi = (p.x - fx0) / (fx1 - fx0) * 199.0;
	float k = 0.0;
	for (int j = -1; j <= 1; j++) {
		highp float i0 = floor(fi) + float(j);
		if (i0 < 0.0 || i0 > 198.0) continue;
		highp vec2 a = vec2(fx0 + (fx1 - fx0) * i0 / 199.0, fy - wave(i0, row) * amp);
		highp vec2 b = vec2(fx0 + (fx1 - fx0) * (i0 + 1.0) / 199.0, fy - wave(i0 + 1.0, row) * amp);
		highp vec2 d = b - a;
		highp float t = clamp(dot(p - a, d) / dot(d, d), 0.0, 1.0);
		float dist = float(length(p - a - d * t));
		k = max(k, clamp(0.8 * u[18] - dist + 0.5, 0.0, 1.0) * 0.85 + clamp(2.25 * u[18] - dist + 0.5, 0.0, 1.0) * 0.18);
	}
	return k;
}

vec3 phosphor(highp vec2 p, highp vec2 tp) {
	vec3 ink = vec3(140.0, 255.0, 214.0) / 255.0;
	vec4 txt = texture2D(t2, tp);
	highp float ly = u[6], fs = u[7], x0 = u[8];
	bool line = abs(p.y - ly) < fs * 1.2;
	if (line && p.x > x0 + u[9]) txt = vec4(0.0);
	vec3 col = txt.rgb;
	if (u[11] > 0.5 && line) {
		highp float cx = x0 + u[9] + u[10] * 0.2;
		if (p.x >= cx && p.x < cx + u[10] * 0.9 && abs(p.y - ly) < fs * 0.5) col += ink * 0.9;
	}
	if (abs(p.y - u[16]) < u[17] * 1.3 && p.x > u[14] - 4.0 && p.x < u[15] + 4.0) {
		float k = 0.0;
		for (int r = 0; r < 4; r++) {
			float a = u[20 + r];
			if (a <= 0.0) continue;
			k = max(k, trace(p, float(r)) * a);
		}
		col += ink * k;
	}
	return col * u[13];
}

void main() {
	highp float s = res.x / u[5];
	highp vec2 p = pixel() / s;
	highp vec2 at = vec2(u[0], u[1]), size = vec2(u[2], u[3]);
	float inside = rounded(p, at, size, u[4]);
#ifdef LIGHT
	gl_FragColor = vec4(inside > 0.0 ? phosphor(p, vtex) * inside : vec3(0.0), 1.0);
#else
	vec3 col = texture2D(t0, vtex).rgb;
	if (inside > 0.0) {
		col = min(col + phosphor(p, vtex) * inside, 1.0);
		vec4 g = texture2D(t1, vtex);
		col = g.rgb + col * (1.0 - g.a);
	}
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
