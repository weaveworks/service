import md5 from 'md5';


export default function gravatar(email) {
  const hash = md5(email);
  return `https://www.gravatar.com/avatar/${hash}?d=mm`;
}

