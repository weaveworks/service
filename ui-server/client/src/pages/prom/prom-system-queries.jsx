import React from 'react';

import Colors from '../../common/colors';

export default class PromSystemQueries extends React.Component {

  render() {
    const { label, queries, onClickCategory, onClickQuery } = this.props;
    const styles = {
      category: {
        marginLeft: 24,
      },
      label: {
        marginTop: 4,
        marginBottom: 4,
        color: Colors.text2,
        textTransform: 'uppercase',
        fontSize: '0.9em',
      },
      query: {
        fontSize: '0.9em',
        textDecoration: 'underline',
        paddingBottom: 4,
        cursor: 'pointer'
      }
    };

    return (
      <div style={styles.category}>
        <div onClick={onClickCategory} style={styles.label}>
          {label}
        </div>
        <div>
          {queries.map(({ label: text, query }) => <div
            style={styles.query}
            onClick={onClickQuery}
            title={query}
            key={text}>{text}</div>)}
        </div>
      </div>
    );
  }

}
