highp float hh(highp vec2 p) {
	highp vec3 p3 = fract(vec3(p.xyx) * 0.1031);
	p3 += dot(p3, p3.yzx + 33.33);
	return fract((p3.x + p3.y) * p3.z);
}

vec3 layer(highp vec2 px, float s, float lrow, float bright, float rising, float rows) {
	float cw = s * 0.62;
	highp vec2 cell = px / vec2(cw, s);
	vec2 id = vec2(floor(cell));
	vec2 local = vec2(fract(cell));
	vec4 st = texture2D(t0, vec2((id.x + 0.5) / 256.0, (lrow + 0.5) / 4.0)) * 255.0;
	highp float h = hh(id + vec2(lrow * 97.0, lrow * 31.0));
	highp float k = floor(u[2] * (0.3 + 2.2 * fract(h * 97.13)) + fract(h * 13.7) * 17.0);
	float idx = floor(float(fract(h * 61.7 + k * 0.618034)) * 63.99);
	vec2 a = vec2(mod(idx, 8.0), floor(idx / 8.0));
	float g = texture2D(t1, (a + vec2(0.2 + local.x * 0.6, 0.06 + local.y * 0.88)) / 8.0).r;
	float d = st.r - id.y;
	float on = st.a / 255.0;
	float fall = step(0.0, d) * step(d, st.g - 1.0) * (1.0 - d / max(st.g, 1.0)) * on;
	float head = step(0.0, d) * step(d, 0.5) * on;
	float up = rows - 1.0 - id.y;
	float lift = step(up, st.b - 1.0) * rising;
	float rb = lift * (0.25 + 0.75 * (up + 1.0) / max(st.b, 1.0));
	float top = lift * step(st.b - 1.5, up);
	vec3 c = vec3(0.75, 1.0, 0.8) * head + vec3(0.75, 1.0, 1.0) * top;
#ifndef LIGHT
	c += vec3(0.1, 0.85, 0.3) * fall * fall + vec3(0.08, 0.6, 0.85) * rb * rb;
#endif
	return c * g * bright;
}

void main() {
	highp vec2 px = pixel();
	float s = u[3];
	vec3 col = layer(px, s * 0.6, 1.0, 0.45, 1.0, u[5]);
	col += layer(px, s, 0.0, 1.0, 1.0, u[4]);
#ifdef LIGHT
	gl_FragColor = vec4(col * 0.5, 1.0);
#else
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
