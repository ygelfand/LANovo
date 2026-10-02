#ifdef PRE
precision highp float;

float top(float x, int i) {
	float W = u[52], H = u[53];
	float tt = x / W * 2.0 - 1.0;
	float pin = 1.0 - tt * tt;
	pin *= pin;
	int b = 1 + 9 * i;
	float d = (tt - u[b + 3]) / u[b + 4];
	float e = exp(-d * d) * pin;
	return u[b] * H * e * sin(u[b + 1] * tt * 6.2831853 + u[b + 2]) * (0.8 + 0.2 * sin(u[b + 5] + x * u[54]));
}

void main() {
	float x = floor(gl_FragCoord.x) + 0.5;
	int row = int(floor(gl_FragCoord.y));
	float t0 = 0.0, t1 = 0.0;
	for (int i = 0; i < 5; i++) {
		if (i == row) {
			t0 = top(x, i);
			t1 = top(x + 1.0, i);
		}
	}
	gl_FragColor = vec4(enc16(t0 / u[53] + 0.5), enc16((t1 - t0) / 20.0 + 0.5));
}
#else

vec3 ribbon(vec3 col, highp vec2 p, highp vec4 c, int i) {
	int b = 1 + 9 * i;
	highp float H = u[53], cy = u[50];
	highp float t = (dec16(c.rg) - 0.5) * H;
	highp float slope = (dec16(c.ba) - 0.5) * 20.0;
	vec3 rc = vec3(u[b + 6], u[b + 7], u[b + 8]);
	highp float y0 = cy - t, y1 = cy + t * 0.86;
	highp float lo = min(y0, y1), hi = max(y0, y1);
	if (p.y >= lo && p.y <= hi) {
		highp float span = max(4.0 * u[55], abs(u[b]) * H * 1.05);
		float r = float((p.y - (cy - span)) / (2.0 * span));
		float a = r < 0.3 ? mix(0.0, 0.22, r / 0.3) : r < 0.5 ? mix(0.22, 0.46, (r - 0.3) / 0.2) : r < 0.7 ? mix(0.46, 0.22, (r - 0.5) / 0.2) : mix(0.22, 0.0, clamp((r - 0.7) / 0.3, 0.0, 1.0));
		col += rc * a;
	}
	float dist = float(abs(p.y - y0) / sqrt(1.0 + slope * slope));
	float k = clamp(0.75 * u[55] - dist + 0.5, 0.0, 1.0);
	col += mix(rc, vec3(1.0), 0.3) * (0.6 + 0.3 * u[51]) * k;
	return col;
}

vec3 scene(highp vec2 p) {
	highp float W = u[52], H = u[53], cy = u[50];
	float d = float(length(p - vec2(W * 0.5, H * 0.6)) / (max(W, H) * 0.6));
	vec3 col = vec3(2.0, 3.0, 8.0) / 255.0;
	col = mix(col, vec3(30.0, 40.0, 80.0) / 255.0, 0.35 * max(1.0 - d, 0.0));
	float bk = clamp(0.7 * u[55] - float(abs(p.y - cy)) + 0.5, 0.0, 1.0);
	float bx = 1.0 - abs(float(p.x / W) * 2.0 - 1.0);
	col += mix(vec3(160.0, 200.0, 255.0), vec3(200.0, 225.0, 255.0), bx) / 255.0 * u[56] * bx * bk;
	if (abs(p.y - cy) > u[57]) return min(col, 1.0);
	highp float xn = p.x / W;
	col = ribbon(col, p, texture2D(pre, vec2(xn, 0.5 / 8.0)), 0);
	col = ribbon(col, p, texture2D(pre, vec2(xn, 1.5 / 8.0)), 1);
	col = ribbon(col, p, texture2D(pre, vec2(xn, 2.5 / 8.0)), 2);
	col = ribbon(col, p, texture2D(pre, vec2(xn, 3.5 / 8.0)), 3);
	col = ribbon(col, p, texture2D(pre, vec2(xn, 4.5 / 8.0)), 4);
	return min(col, 1.0);
}

void main() {
	highp vec2 p = pixel() * (u[52] / res.x);
#ifdef LIGHT
	gl_FragColor = vec4(scene(p), 1.0);
#else
	gl_FragColor = vec4(min(scene(p) + glow(), 1.0), 1.0);
#endif
}
#endif
