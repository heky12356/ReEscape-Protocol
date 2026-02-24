export function InputField({
  label,
  value,
  onChange,
  type = "text",
  step,
  placeholder
}) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      <input
        className="field-input"
        type={type}
        step={step}
        value={value ?? ""}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

export function SelectField({ label, value, options, onChange }) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      <select
        className="field-input"
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
      >
        {(options || []).map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

export function TextAreaField({ label, value, onChange, rows = 10 }) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      <textarea
        className="field-textarea"
        rows={rows}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}
