float screen(highp vec2 p) {
	highp vec2 at = p;
	highp vec2 l = p - vec2(u[16], u[17]);
	if (l.x >= 0.0 && l.y >= 0.0 && l.x < u[18] && l.y < u[19] && floor(l.y / u[20]) == u[21]) at = vec2(l.x, u[1] + l.y);
	float lum = texture2D(t0, at / vec2(u[0], u[24])).r;
	highp vec2 b = p - vec2(u[10], u[11]);
	highp vec4 w = texture2D(t1, vec2((clamp(b.x / u[12], 0.0, 1.0) * 127.0 + 0.5) / 128.0, 0.5));
	if (b.x >= 0.0 && b.y >= 0.0 && b.x < u[12] && b.y < u[13]) {
		highp float y0 = (w.r * 65280.0 + w.g * 255.0) / 65535.0 * u[13];
		highp float slope = ((w.b * 65280.0 + w.a * 255.0) / 65535.0 - 0.5) * u[25];
		float d = float(abs(b.y - y0) * inversesqrt(1.0 + slope * slope));
		if (d < 40.0 * u[22]) lum += clamp(u[15] - d + 0.5, 0.0, 1.0) + 0.35 * exp(-d / (5.0 * u[22]));
	}
	return lum;
}

void main() {
	highp vec2 c = vtex * 2.0 - 1.0;
	highp vec2 q = c * (1.0 + u[2] * dot(c, c));
	highp vec2 uv = q * 0.5 + 0.5;
	vec2 edge = vec2(min(uv, 1.0 - uv));
	if (edge.x < 0.0 || edge.y < 0.0) {
		gl_FragColor = vec4(0.0, 0.0, 0.0, 1.0);
		return;
	}
	highp vec2 p = uv * vec2(u[0], u[1]);
	vec3 tint = vec3(u[6], u[7], u[8]);
	float lum = min(screen(p), 1.3);
#ifdef LIGHT
	gl_FragColor = vec4(tint * lum * 0.8, 1.0);
#else
	float sl = float(abs(fract(p.y / u[3]) - 0.5)) * 2.0;
	float rb = clamp(1.0 - abs(float(fract(uv.y - u[5] * 0.06)) - 0.08) * 12.5, 0.0, 1.0);
	vec2 vc = vec2(c);
	float k = (1.0 - u[4] * sl) * (1.0 + 0.07 * u[23] * rb) * u[9];
	k *= clamp(edge.x * 83.0, 0.0, 1.0) * clamp(edge.y * 83.0, 0.0, 1.0) * (1.0 - 0.28 * dot(vc, vc));
	gl_FragColor = vec4(min((tint * (lum + 0.05) + glow() * tint) * k, 1.0), 1.0);
#endif
}
