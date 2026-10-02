const float P = 72.0;

highp vec2 ridgeAt(highp float xn, float row) {
	highp float fi = clamp(xn, 0.0, 1.0) * (P - 1.0);
	highp float i0 = min(floor(fi), P - 2.0);
	vec4 a = texture2D(t1, vec2((i0 + 0.5) / 256.0, (row + 0.5) / 2.0));
	vec4 b = texture2D(t1, vec2((i0 + 1.5) / 256.0, (row + 0.5) / 2.0));
	highp float ha = dec16(a.rg) * 2.0, hb = dec16(b.rg) * 2.0;
	return vec2(mix(ha, hb, fi - i0), hb - ha);
}

vec3 ridge(vec3 col, highp vec2 p, float row, highp float hmax, vec3 fillTop, vec3 fillBottom, vec3 edge, float alpha, float width) {
	highp float hz = u[0], W = u[14];
	if (p.y < hz - hmax * 2.0 - width) return col;
	highp float xn = p.x / W;
	highp vec2 r = ridgeAt(xn, row);
	highp float y = hz - r.x * hmax;
	highp float slope = -r.y * hmax * (P - 1.0) / W;
	if (p.y >= y && p.y <= hz) {
		float t = clamp(float((p.y - (hz - u[15] * 0.18)) / (u[15] * 0.18)), 0.0, 1.0);
		col = mix(fillTop, fillBottom, t);
	}
	float dist = float(abs(p.y - y) / sqrt(1.0 + slope * slope));
	float k = clamp(width * 0.5 - dist + 0.5, 0.0, 1.0);
	return mix(col, edge, k * alpha);
}

vec3 scene(highp vec2 p) {
	highp float hz = u[0], cx = u[1], cy = u[2], rs = u[3], uu = u[5];
	float lv = u[4];
	vec3 neon = vec3(u[6], u[7], u[8]), neon2 = vec3(u[9], u[10], u[11]);
	vec4 base = texture2D(t0, vtex);
	vec3 col = base.rgb;
	if (p.y < hz) {
		highp float d = length(p - vec2(cx, cy));
		if (d < rs * 2.4) {
			float g = clamp(float((d - rs * 0.6) / (rs * 1.8)), 0.0, 1.0);
			col += vec3(1.0, 90.0 / 255.0, 160.0 / 255.0) * (0.22 + 0.25 * lv) * (1.0 - g);
		}
		float sk = clamp(float(rs - d) + 0.5, 0.0, 1.0);
		if (sk > 0.0) {
			bool cut = false;
			for (int k = 0; k < 7; k++) {
				highp float sy = cy + rs * (0.02 + float(k) * 0.155);
				highp float th = rs * (0.018 + 0.028 * float(k) / 6.0) * u[12];
				if (p.y >= sy && p.y < sy + th) cut = true;
			}
			if (!cut) {
				float t = float((p.y - (cy - rs)) / (2.0 * rs));
				vec3 s = t < 0.45 ? mix(vec3(1.0, 242.0 / 255.0, 122.0 / 255.0), vec3(1.0, 176.0 / 255.0, 58.0 / 255.0), t / 0.45)
					: t < 0.75 ? mix(vec3(1.0, 176.0 / 255.0, 58.0 / 255.0), vec3(1.0, 79.0 / 255.0, 122.0 / 255.0), (t - 0.45) / 0.3)
					: mix(vec3(1.0, 79.0 / 255.0, 122.0 / 255.0), vec3(214.0 / 255.0, 30.0 / 255.0, 143.0 / 255.0), (t - 0.75) / 0.25);
				col = mix(col, s, sk);
			}
		}
		col = ridge(col, p, 1.0, u[15] * 0.2, vec3(18.0, 4.0, 42.0) / 255.0, vec3(18.0, 4.0, 42.0) / 255.0, neon2, 0.55, 1.2 * uu);
		col = ridge(col, p, 0.0, u[15] * 0.16, vec3(29.0, 6.0, 54.0) / 255.0, vec3(11.0, 2.0, 24.0) / 255.0, neon, 0.95, 1.8 * uu);
	}
	float hl = clamp(0.8 * uu - float(abs(p.y - hz)) + 0.5, 0.0, 1.0);
	col += mix(neon, vec3(1.0), 0.4) * 0.9 * hl;
	if (p.y > hz) {
		highp float camH = u[15] - hz;
		highp float dy = p.y - hz;
		highp float z = camH / dy;
		float lw = 1.4 * uu;
		float line = 0.0;
		if (z < 16.0 && z > 0.09) {
			highp float f = fract(z + u[13]);
			highp float dz = min(f, 1.0 - f);
			float px = float(dz * dy * dy / camH);
			line = clamp(lw * 0.5 - px + 0.5, 0.0, 1.0);
		}
		vec3 dyn = neon * 0.8 * line;
		for (int i = 0; i < 6; i++) {
			float wa = u[21 + 2 * i];
			if (wa <= 0.0) continue;
			highp float wy = u[20 + 2 * i];
			float t = float(abs(p.y - wy) / (10.0 * uu));
			if (t < 1.0) dyn += mix(neon2, vec3(1.0), 0.35) * wa * (1.0 - t);
		}
		float ht = float(dy / (camH * 0.35));
		float keep = ht < 1.0 && dy > 0.8 * uu ? 1.0 - 0.9 * (1.0 - ht) : 1.0;
		col = min(col + neon * 0.8 * base.a + dyn * keep, 1.0);
	}
	return min(col, 1.0);
}

void main() {
	highp float s = res.x / u[14];
	highp vec2 p = pixel() / s;
#ifdef LIGHT
	gl_FragColor = vec4(scene(p), 1.0);
#else
	gl_FragColor = vec4(min(scene(p) + glow(), 1.0), 1.0);
#endif
}
